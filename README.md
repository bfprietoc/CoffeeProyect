# CoffeeProyect — Backend

API REST en Go para una tienda de café especializado. Soporta modo in-memory (sin infra) y modo completo con PostgreSQL + Redis.

---

## Prerrequisitos

| Herramienta | Instalación |
|---|---|
| Go 1.23+ | `brew install go` |
| Colima | `brew install colima` |
| golang-migrate | `brew install golang-migrate` |

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

### 4. Correr migraciones

```bash
make migrate-up
```

Aplica las 9 migraciones en orden. Si ya estaban aplicadas imprime `no change` — está bien.

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
pg_notify listener started
listening on :8080
```

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
| `SMTP_HOST` | Host SMTP para emails | emails deshabilitados |

Valor local por defecto:
```
DATABASE_URL=postgres://coffee:coffee@localhost:5432/coffeedb?sslmode=disable
REDIS_URL=redis://localhost:6379
```

---

## Credenciales de prueba

| Qué | Valor |
|---|---|
| Usuario registrado | `test2@coffee.com` / `password123` |
| Admin API Key | `secret123` |

Ver endpoints y ejemplos curl detallados en [EXECUTION.md](./EXECUTION.md).
