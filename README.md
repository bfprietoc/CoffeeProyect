# CoffeeProyect — Backend

API REST en Go para una tienda de café especializado. Soporta modo in-memory (sin infra) y modo completo con PostgreSQL + Redis.

---

## Prerrequisitos

| Herramienta | Instalación (macOS) | Nota |
|---|---|---|
| Go 1.23+ | `brew install go` | — |
| Docker / Colima | `brew install colima` | En Linux, Docker directo (sin Colima) |
| golang-migrate | `brew install golang-migrate` | **Opcional**: solo para los targets `make migrate-up/down`. El servidor aplica las migraciones automáticamente al arrancar con `DATABASE_URL` |

---

## Setup inicial (solo la primera vez)

```bash
cp .env.example .env
go mod download
```

El `.env` por defecto apunta a la DB local de Docker. Sin `DATABASE_URL` el servidor arranca en modo in-memory.

---

## Opción A — Sin Docker (in-memory, más rápido)

No requiere Colima ni base de datos. Los datos se pierden al reiniciar.

```bash
# Comentar o borrar DATABASE_URL y REDIS_URL en .env
make run
```

Verificar:
```bash
curl http://localhost:8080/health
curl http://localhost:8080/coffees
```

---

## Opción B — Con Docker vía Colima (flujo completo)

### 1. Iniciar Colima

```bash
colima start
```

### 2. Apuntar Docker al socket de Colima

Si tenés Docker Desktop instalado, la variable `DOCKER_HOST` puede estar apuntando al socket de Desktop.
Sobreescribila para que todos los comandos usen Colima:

```bash
export DOCKER_HOST="unix://$HOME/.colima/default/docker.sock"
```

> Tip: agregá esta línea a tu `~/.zshrc` o `~/.bashrc` para no repetirla en cada sesión.

### 3. Levantar PostgreSQL + Redis

```bash
make infra-up
```

Espera a que Postgres esté listo antes de continuar (el Makefile lo hace automáticamente).

### 4. Correr migraciones (opcional)

```bash
make migrate-up
```

> **Nota:** este paso es opcional. El servidor aplica las migraciones automáticamente al arrancar con `DATABASE_URL` (ver `internal/db/migrate.go`). Úsalo si querés aplicarlas antes de levantar el servidor o revertir con `make migrate-down` (ambos targets requieren el binario `golang-migrate`).

### 5. Cargar datos de prueba

```bash
make seed
```

Inserta 5 cafés colombianos con IDs fijos. Es idempotente: correrlo dos veces no duplica datos.

### 6. Levantar el servidor

```bash
make run
```

Log esperado:
```
connected to PostgreSQL
connected to Redis (guest carts)
Redis catalog cache enabled
SMTP_HOST not set — email notifications disabled
pg_notify listener started
listening on :8080
```

> **En Linux:** no hace falta Colima — basta con Docker (`sudo systemctl start docker` si el daemon no está corriendo). Los pasos 1 y 2 quedan en ese caso.

---

## Detener la infraestructura

```bash
make infra-down
```

El volumen de PostgreSQL persiste. Para borrarlo también:
```bash
DOCKER_HOST="unix://$HOME/.colima/default/docker.sock" docker compose down -v
```

---

## Referencia del Makefile

| Comando | Qué hace |
|---|---|
| `make run` | Levanta el servidor (lee `.env`) |
| `make test` | Tests unitarios (sin Docker) |
| `make test-integration` | Tests de integración (requiere DB) |
| `make infra-up` | Levanta PostgreSQL + Redis en Docker |
| `make infra-down` | Baja los contenedores |
| `make migrate-up` | Aplica migraciones pendientes |
| `make migrate-down` | Revierte la última migración |
| `make seed` | Carga los 5 cafés colombianos |

---

## Variables de entorno (`.env`)

| Variable | Descripción | Default si falta |
|---|---|---|
| `PORT` | Puerto HTTP | `8080` |
| `DATABASE_URL` | DSN de PostgreSQL | usa in-memory |
| `REDIS_URL` | URL de Redis | usa in-memory |
| `JWT_SECRET` | Secreto para firmar JWT | `dev-secret-change-in-production` |
| `ADMIN_API_KEY` | API key para endpoints `/admin/*` | admin endpoints deshabilitados |
| `WOMPI_PUBLIC_KEY` | Llave pública de comercio Wompi (`pub_test_…`) | pagos deshabilitados (503) |
| `WOMPI_INTEGRITY_SECRET` | Secreto de integridad Wompi (server-side) | pagos deshabilitados |
| `WOMPI_EVENT_SECRET` | Secreto de eventos Wompi (distinto al de integridad) | pagos deshabilitados |
| `PUBLIC_BASE_URL` | URL pública del sitio (redirect del checkout) | pagos deshabilitados |
| `GOOGLE_CLIENT_ID` | OAuth Client ID de Google para `POST /auth/google` | login con Google deshabilitado (503) |
| `SMTP_HOST` | Host SMTP para emails | emails deshabilitados |

Valor local por defecto:
```
DATABASE_URL=postgres://coffee:coffee@localhost:5432/coffeedb?sslmode=disable
REDIS_URL=redis://localhost:6379
```

### Pagos (Wompi Web Checkout)

Los pagos usan el **Web Checkout** de Wompi (sin PCI scope en nuestra app): el
backend valida el pedido y firma los parámetros del formulario de Wompi
(nunca expone secretos), el cliente paga en el portal de Wompi y el backend
sabe el resultado vía **webhook firmado** (`wompi_hash` no aplica; el checksum
del evento se calcula del propio evento).

- `POST /orders/{id}/payment` (auth, dueño) — valida el pedido, genera una
  referencia única por intento (`{order_id}-{unix}`) y devuelve los parámetros
  del checkout (`{publicKey, reference, amountInCents, currency, signature,
  redirectUrl, customerEmail}`). El frontend renderiza el formulario de
  Wompi con esos valores.
- `POST /webhooks/wompi` (público, verificado por checksum) — valida la
  firma del evento (SHA256 de las propiedades del evento + timestamp +
  secreto de eventos), marca el pedido `paid`/`failed` (idempotente por
  referencia) y responde 200 (Wompi reintenta hasta 3 veces en 24h ante
  cualquier otro status).
- Admin: un pedido solo puede pasar a `processing` si está `paid`
  (`PAYMENT_REQUIRED`, 422). Los pedidos creados antes de esta feature
  migran como `paid` (se pagaban fuera del flujo transaccional).
- Errores: `PAYMENTS_DISABLED` (503, faltan credenciales), `NOT_FOUND` (404),
  `PAYMENT_ALREADY_PAID` (409), `PAYMENT_UNAVAILABLE` (422, pedido cancelado).

**Regla:** el frontend NUNCA arma la firma de integridad; el backend la
computa siempre. Ver `internal/payment/wompi.go`.

---

## Credenciales de prueba

| Qué | Valor |
|---|---|
| Usuario registrado | `test2@coffee.com` / `password123` |
| Admin API Key | `secret123` |

Ver endpoints y ejemplos curl detallados en [EXECUTION.md](./EXECUTION.md).
