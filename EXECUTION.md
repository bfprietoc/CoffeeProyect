# CoffeeProyect — Guía de Ejecución

## Flujos de prueba rápida

### Opción A — Sin Docker (in-memory, más rápido)

No requiere ninguna infraestructura. Los datos se pierden al reiniciar.

```bash
make run
# o: go run main.go
```

```bash
# Verificar que levantó
curl http://localhost:8080/health

# Listar catálogo (5 cafés colombianos seeded)
curl http://localhost:8080/coffees

# Registrarse
curl -X POST http://localhost:8080/auth/register \
  -H "Content-Type: application/json" \
  -d '{"name":"Brayan","email":"brayan@test.com","password":"password123"}'

# Guardar el token
TOKEN="<access_token del response>"

# Agregar al carrito y hacer checkout
curl -X POST http://localhost:8080/cart/items \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"coffee_id":"c1a2b3c4-0001-0001-0001-000000000001","quantity":1}'
```

### Opción B — Con Docker (PostgreSQL + Redis, flujo completo)

```bash
make infra-up          # levanta Postgres + Redis
make migrate-up        # corre las 9 migraciones
make seed              # carga los 5 cafés colombianos
ADMIN_API_KEY=secret make run
```

```bash
# Tests de integración (incluye test de concurrencia con FOR UPDATE)
DATABASE_URL=postgres://coffee:coffee@localhost:5432/coffeedb?sslmode=disable \
  go test ./internal/store/postgres/... -tags integration -v
# o simplemente:
make test-integration
```

### Opción C — Solo unit tests (sin infra)

```bash
make test
# → ok handler, ok service, ok store/memory — 0 fallos, sin Docker
```

---

## Requisitos previos

| Herramienta | Versión mínima | Verificar |
|---|---|---|
| Go | 1.23+ | `go version` |
| Docker Desktop | cualquiera | `docker --version` |

---

## Setup inicial (primera vez)

```bash
# 1. Clonar el repo y entrar
git clone <repo-url>
cd CoffeeProyect

# 2. Copiar variables de entorno
cp .env.example .env

# 3. Descargar dependencias Go
go mod download
```

---

## Levantar infraestructura (Docker)

```bash
# Levantar PostgreSQL y Redis en background
make infra-up
# equivalente a: docker compose up -d

# Detener y eliminar los contenedores
make infra-down
# equivalente a: docker compose down

# Ver estado de los contenedores
docker compose ps

# Ver logs de postgres
docker compose logs postgres

# Ver logs de redis
docker compose logs redis
```

---

## Migraciones de base de datos

> Requiere que los contenedores estén corriendo (`make infra-up`).

```bash
# Aplicar todas las migraciones pendientes
make migrate-up

# Revertir la última migración
make migrate-down

# Cargar datos de prueba (seed)
make seed
```

### Setup completo de DB en un solo flujo

```bash
make infra-up && make migrate-up && make seed
```

---

## Correr el servidor

### Sin base de datos (modo in-memory)

Útil para desarrollo rápido sin levantar Docker.

```bash
make run
# equivalente a: go run main.go
```

El servidor arranca en `http://localhost:8080` con 5 cafés de prueba en memoria.
Al reiniciar se pierden los datos — es intencional.

### Con base de datos PostgreSQL

```bash
# Asegurarse de tener .env con DATABASE_URL configurado
make infra-up && make migrate-up && make seed

# Correr el servidor (lee DATABASE_URL desde .env via Makefile)
make run
```

Log esperado al arrancar con DB + Redis + SMTP:
```
2026/05/01 18:00:00 connected to PostgreSQL
2026/05/01 18:00:00 connected to Redis (guest carts)
2026/05/01 18:00:00 Redis catalog cache enabled
2026/05/01 18:00:00 pg_notify listener started
2026/05/01 18:00:00 listening on :8080
```

Log si falta ADMIN_API_KEY o SMTP_HOST:
```
2026/05/01 18:00:00 WARNING: ADMIN_API_KEY not set — admin endpoints are disabled
2026/05/01 18:00:00 SMTP_HOST not set — email notifications disabled
```

### Emails automáticos

Cuando `SMTP_HOST` está configurado, el servidor envía emails automáticamente ante estos eventos:

| Trigger | Email enviado |
|---|---|
| `POST /auth/register` | Bienvenida al nuevo usuario |
| `POST /orders` (checkout) | Confirmación de pedido con resumen de ítems |
| `PATCH /admin/orders/{id}/status` → `shipped` | Pedido en camino + número de tracking |
| `PATCH /admin/orders/{id}/status` → `delivered` | Pedido entregado |
| `POST /orders/{id}/cancel` | Orden cancelada |
| `PATCH /admin/coffees/{id}/stock` (stock sube de 0) | Aviso a todos en la waitlist |
| `POST /waitlist` | Confirmación de suscripción a la waitlist |

Log esperado al arrancar con DB sin Redis:
```
2026/05/01 18:00:00 connected to PostgreSQL
2026/05/01 18:00:00 REDIS_URL not set — using in-memory guest cart store
2026/05/01 18:00:00 listening on :8080
```

Log esperado al arrancar sin DB:
```
2026/05/01 18:00:00 DATABASE_URL not set — using in-memory store
2026/05/01 18:00:00 listening on :8080
```

---

## Tests

### Tests unitarios (no requieren Docker)

Corren siempre, sin base de datos. Cubren el store in-memory y los handlers HTTP.

```bash
# Todos los tests unitarios
make test
# equivalente a: go test ./...

# Con output detallado
go test ./... -v

# Un paquete específico
go test ./internal/store/memory/... -v
go test ./internal/handler/... -v
```

### Tests de integración (requieren PostgreSQL corriendo)

Están aislados con el build tag `integration` y se saltean si no hay `DATABASE_URL`.

```bash
# Setup previo (solo la primera vez o tras make infra-down)
make infra-up && make migrate-up && make seed

# Correr tests de integración del store PostgreSQL
make test-integration
# equivalente a:
go test ./internal/store/postgres/... -tags integration -v
```

### Correr todo junto

```bash
# Unitarios + integración en un solo comando
make infra-up && make migrate-up && make seed && make test && make test-integration
```

---

## Endpoints disponibles

> Servidor corriendo en `http://localhost:8080`

### Catálogo (públicos)

```bash
# Health check
curl http://localhost:8080/health

# Listar todos los cafés
curl http://localhost:8080/coffees

# Listar solo los disponibles
curl "http://localhost:8080/coffees?available=true"

# Filtrar por roast level
curl "http://localhost:8080/coffees?roast_level=light"

# Filtrar por país y proceso
curl "http://localhost:8080/coffees?country=Colombia&process=natural"

# Paginación
curl "http://localhost:8080/coffees?page=1&limit=2"

# Detalle de un café por ID
curl http://localhost:8080/coffees/c1a2b3c4-0001-0001-0001-000000000001

# Respuesta de error (ID inexistente)
curl http://localhost:8080/coffees/no-existe
```

### IDs de los cafés en el seed

| ID | Nombre |
|---|---|
| `c1a2b3c4-0001-0001-0001-000000000001` | El Paraíso 92 |
| `c1a2b3c4-0002-0002-0002-000000000002` | Finca La Esperanza Wush Wush |
| `c1a2b3c4-0003-0003-0003-000000000003` | Huila Natural Caturra |
| `c1a2b3c4-0004-0004-0004-000000000004` | Nariño Honey Gesha |
| `c1a2b3c4-0005-0005-0005-000000000005` | Antioquia Dark Roast Blend (sin stock) |

### Autenticación

```bash
# Registrar usuario
curl -X POST http://localhost:8080/auth/register \
  -H "Content-Type: application/json" \
  -d '{"name":"Brayan","email":"brayan@test.com","password":"password123"}'

# Login
curl -X POST http://localhost:8080/auth/login \
  -H "Content-Type: application/json" \
  -d '{"email":"brayan@test.com","password":"password123"}'

# Guardar el access token para usarlo en las siguientes llamadas
TOKEN="<access_token del login>"

# Refresh de token
curl -X POST http://localhost:8080/auth/refresh \
  -H "Content-Type: application/json" \
  -d '{"refresh_token":"<refresh_token>"}'
```

### Perfil de usuario (requieren JWT)

```bash
# Ver perfil
curl http://localhost:8080/users/me \
  -H "Authorization: Bearer $TOKEN"

# Actualizar nombre
curl -X PUT http://localhost:8080/users/me \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"name":"Brayan Prieto"}'

# Agregar dirección
curl -X POST http://localhost:8080/users/me/addresses \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "label": "casa",
    "street": "Calle 123 #45-67",
    "city": "Bogotá",
    "department": "Cundinamarca",
    "country": "CO",
    "is_default": true
  }'

# Listar direcciones
curl http://localhost:8080/users/me/addresses \
  -H "Authorization: Bearer $TOKEN"

# Eliminar dirección
curl -X DELETE http://localhost:8080/users/me/addresses/<address-id> \
  -H "Authorization: Bearer $TOKEN"
```

---

## Variables de entorno

| Variable | Descripción | Default |
|---|---|---|
| `PORT` | Puerto del servidor HTTP | `8080` |
| `DATABASE_URL` | DSN de PostgreSQL | — (usa in-memory si no está) |
| `REDIS_URL` | URL de Redis para carritos de invitados y caché de catálogo | — (si no está, usa in-memory) |
| `JWT_SECRET` | Secreto para firmar JWT | `dev-secret-change-in-production` (con warning) |
| `ADMIN_API_KEY` | API key para endpoints `/admin/*` | — (si no está, endpoints admin responden 401) |
| `SMTP_HOST` | Host del servidor SMTP para emails | — (si no está, emails deshabilitados silenciosamente) |
| `SMTP_PORT` | Puerto del servidor SMTP | `587` |
| `SMTP_USER` | Usuario SMTP (para PlainAuth) | — |
| `SMTP_PASS` | Contraseña SMTP | — |
| `SMTP_FROM` | Dirección de origen de los emails | — |

Formato de `DATABASE_URL`:
```
postgres://usuario:password@host:puerto/nombre_db?sslmode=disable
```

Valor local por defecto (docker-compose):
```
postgres://coffee:coffee@localhost:5432/coffeedb?sslmode=disable
```

---

## Referencia rápida del Makefile

```bash
make run              # Correr el servidor
make test             # Tests unitarios
make test-integration # Tests de integración (requiere DB)
make migrate-up       # Aplicar migraciones
make migrate-down     # Revertir última migración
make seed             # Cargar datos de prueba
make infra-up         # Levantar Docker (postgres + redis)
make infra-down       # Bajar Docker
```

---

## Estructura de respuestas

### Éxito — producto individual

```json
{
  "id": "c1a2b3c4-0001-0001-0001-000000000001",
  "name": "El Paraíso 92",
  "process": "anaerobic",
  "roast_level": "light",
  "tasting_notes": ["maracuyá", "uva", "chocolate negro"],
  "description": "...",
  "producer": { "id": "...", "name": "Diego Samuel Bermúdez" },
  "farm": { "id": "...", "name": "El Paraíso", "country": "Colombia", "region": "Cauca" },
  "bag_size_grams": 250,
  "price_cents": 8500000,
  "currency": "COP",
  "available": true,
  "stock_bags": 12
}
```

### Error

```json
{
  "error": "coffee not found",
  "code": "NOT_FOUND"
}
```

### Carrito (invitados y usuarios)

```bash
# El carrito funciona con X-Session-ID (invitado) o Authorization: Bearer (usuario)
SESSION="mi-session-uuid-unico"

# Ver carrito vacío
curl http://localhost:8080/cart -H "X-Session-ID: $SESSION"

# Agregar café al carrito
curl -X POST http://localhost:8080/cart/items \
  -H "X-Session-ID: $SESSION" \
  -H "Content-Type: application/json" \
  -d '{"coffee_id":"c1a2b3c4-0001-0001-0001-000000000001","quantity":2}'

# Cambiar cantidad
curl -X PATCH http://localhost:8080/cart/items/c1a2b3c4-0001-0001-0001-000000000001 \
  -H "X-Session-ID: $SESSION" \
  -H "Content-Type: application/json" \
  -d '{"quantity":3}'

# Quitar un ítem
curl -X DELETE http://localhost:8080/cart/items/c1a2b3c4-0001-0001-0001-000000000001 \
  -H "X-Session-ID: $SESSION"

# Vaciar carrito
curl -X DELETE http://localhost:8080/cart -H "X-Session-ID: $SESSION"

# Usar carrito como usuario autenticado
curl http://localhost:8080/cart -H "Authorization: Bearer $TOKEN"

# Login con fusión de carrito invitado → se pasa X-Session-ID para mergear
curl -X POST http://localhost:8080/auth/login \
  -H "Content-Type: application/json" \
  -H "X-Session-ID: $SESSION" \
  -d '{"email":"brayan@test.com","password":"password123"}'
```

### Checkout y órdenes (requieren JWT)

```bash
# Guardar dirección primero
curl -X POST http://localhost:8080/users/me/addresses \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "label": "casa",
    "street": "Calle 123 #45-67",
    "city": "Bogotá",
    "department": "Cundinamarca",
    "country": "CO",
    "is_default": true
  }'

ADDRESS_ID="<id de la dirección>"

# Validar carrito antes de comprar
curl http://localhost:8080/checkout/validate \
  -H "Authorization: Bearer $TOKEN"

# Crear orden (descuenta stock atómicamente)
curl -X POST http://localhost:8080/orders \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d "{\"address_id\": \"$ADDRESS_ID\"}"

ORDER_ID="<id de la orden>"

# Ver historial de órdenes
curl http://localhost:8080/orders \
  -H "Authorization: Bearer $TOKEN"

# Ver detalle de una orden
curl http://localhost:8080/orders/$ORDER_ID \
  -H "Authorization: Bearer $TOKEN"

# Cancelar orden (solo si está en confirmed o processing)
curl -X POST http://localhost:8080/orders/$ORDER_ID/cancel \
  -H "Authorization: Bearer $TOKEN"
```

### Waitlist (optionalAuth — funciona con o sin JWT)

```bash
# Suscribirse a un café sin stock (sin autenticar)
curl -X POST http://localhost:8080/waitlist \
  -H "Content-Type: application/json" \
  -d '{"coffee_id":"c1a2b3c4-0005-0005-0005-000000000005","email":"fan@example.com"}'

# Suscribirse autenticado (asocia la entrada al usuario)
curl -X POST http://localhost:8080/waitlist \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"coffee_id":"c1a2b3c4-0005-0005-0005-000000000005","email":"brayan@test.com"}'

# Intentar suscribirse a un café con stock → 422
curl -X POST http://localhost:8080/waitlist \
  -H "Content-Type: application/json" \
  -d '{"coffee_id":"c1a2b3c4-0001-0001-0001-000000000001","email":"fan@example.com"}'
```

### Admin — CRUD de cafés (requiere X-Api-Key)

```bash
ADMIN_KEY="tu-api-key-secreta"

# Crear un café nuevo
curl -X POST http://localhost:8080/admin/coffees \
  -H "X-Api-Key: $ADMIN_KEY" \
  -H "Content-Type: application/json" \
  -d '{
    "name": "Tolima Honey Bourbon",
    "process": "honey",
    "roast_level": "light",
    "description": "Bourbon honey del Tolima.",
    "tasting_notes": ["durazno","miel","mandarina"],
    "bag_size_grams": 250,
    "price_cents": 7200000,
    "currency": "COP",
    "stock_bags": 10,
    "producer_name": "Familia López",
    "farm_name": "Finca El Bosque",
    "farm_country": "Colombia",
    "farm_region": "Tolima"
  }'

NEW_COFFEE_ID="<id devuelto por el POST>"

# Actualizar un café (preserva stock, productor y finca)
curl -X PUT http://localhost:8080/admin/coffees/$NEW_COFFEE_ID \
  -H "X-Api-Key: $ADMIN_KEY" \
  -H "Content-Type: application/json" \
  -d '{
    "name": "Tolima Honey Bourbon Reserva",
    "process": "honey",
    "roast_level": "light",
    "price_cents": 8000000,
    "bag_size_grams": 250
  }'

# Eliminar un café
curl -X DELETE http://localhost:8080/admin/coffees/$NEW_COFFEE_ID \
  -H "X-Api-Key: $ADMIN_KEY"
```

### Admin — stock (requiere X-Api-Key)

```bash
ADMIN_KEY="tu-api-key-secreta"
COFFEE_ID="c1a2b3c4-0005-0005-0005-000000000005"

# Ver stock y movimientos
curl http://localhost:8080/admin/coffees/$COFFEE_ID/stock \
  -H "X-Api-Key: $ADMIN_KEY"

# Reponer stock (dispara stock.replenished y stock.restored si venía de 0)
curl -X PATCH http://localhost:8080/admin/coffees/$COFFEE_ID/stock \
  -H "X-Api-Key: $ADMIN_KEY" \
  -H "Content-Type: application/json" \
  -d '{"operation":"add","quantity":20,"note":"reposición mensual"}'

# Reducir stock (merma, descarte)
curl -X PATCH http://localhost:8080/admin/coffees/$COFFEE_ID/stock \
  -H "X-Api-Key: $ADMIN_KEY" \
  -H "Content-Type: application/json" \
  -d '{"operation":"subtract","quantity":2,"note":"bolsas dañadas"}'
```

### Admin — órdenes (requiere X-Api-Key)

```bash
# Listar todas las órdenes
curl http://localhost:8080/admin/orders \
  -H "X-Api-Key: $ADMIN_KEY"

# Cambiar estado de una orden
curl -X PATCH http://localhost:8080/admin/orders/$ORDER_ID/status \
  -H "X-Api-Key: $ADMIN_KEY" \
  -H "Content-Type: application/json" \
  -d '{"status":"processing"}'

# Marcar como enviada (con número de tracking)
curl -X PATCH http://localhost:8080/admin/orders/$ORDER_ID/status \
  -H "X-Api-Key: $ADMIN_KEY" \
  -H "Content-Type: application/json" \
  -d '{"status":"shipped","tracking_number":"TK-12345-CO"}'
```

---

### Códigos de error

| Code | HTTP | Cuándo |
|---|---|---|
| `NOT_FOUND` | 404 | Recurso inexistente |
| `BAD_REQUEST` | 400 | Parámetro o body inválido |
| `UNAUTHORIZED` | 401 | Token ausente, inválido, expirado o credenciales incorrectas |
| `EMAIL_CONFLICT` | 409 | Email ya registrado |
| `UNAVAILABLE` | 422 | Café sin stock, no se puede agregar al carrito |
| `INVALID_INPUT` | 422 | Input válido pero con restricción de negocio (p.ej. suscribir a waitlist un café con stock) |
| `INSUFFICIENT_STOCK` | 422 | Stock insuficiente para completar la orden |
| `INTERNAL_ERROR` | 500 | Error inesperado del servidor |
