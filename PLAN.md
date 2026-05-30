# CoffeeProyect — Plan de Ejecución

> Cada fase va en su propia rama de feature y se mergea a `develop` antes de avanzar.
> Marcar cada ítem con `[x]` cuando esté completo.
> Las fases están ordenadas por dependencia: cada una se apoya en la anterior.

---

## Fase 0 — Fundamentos ✅

- [x] Inicializar módulo Go (`coffeeproyect`)
- [x] Servidor HTTP con `net/http`
- [x] `GET /health` funcional
- [x] Modelo de dominio `Coffee`, `Producer`, `Farm`
- [x] Interface `CoffeeStore` con `List` y `GetByID`

---

## Fase 1 — Catálogo de productos (in-memory) ✅

> Rama: `feature/product-detail`
> Objetivo: API de solo lectura funcional sin base de datos. Valida el contrato de la API antes de agregar infra.

### Store in-memory
- [x] Implementar `store/memory/coffee_store.go` con `List` y `GetByID`
- [x] Cargar datos de prueba (5 cafés colombianos con todos los campos)
- [x] `ErrNotFound` devuelto correctamente cuando no existe el ID

### Service layer
- [x] Crear `internal/service/coffee_service.go`
- [x] Método `GetByID(id string) (Coffee, error)`
- [x] Método `List(filters CoffeeFilters) ([]Coffee, error)` con filtros `available`, `roast_level`, `country`, `process`
- [x] Definir `CoffeeFilters` struct con campos opcionales

### Handlers HTTP
- [x] Crear `internal/handler/coffee_handler.go`
- [x] `GET /coffees/{id}` → 200 con JSON, 404 si no existe
- [x] `GET /coffees` → 200 con array JSON + query params `?available=true&roast_level=light&page=1&limit=20`
- [x] Crear `internal/handler/health_handler.go` (movido de `routes/`)
- [x] Crear `internal/handler/response.go` con helpers `writeJSON` y `writeError`

### Wiring
- [x] Actualizar `main.go` para inyectar store → service → handler
- [x] Registrar rutas en el mux con método explícito (`GET /coffees`, `GET /coffees/{id}`)

### Errores y respuestas
- [x] Estructura de error estándar: `{"error": "not found", "code": "NOT_FOUND"}`
- [x] Crear `internal/domain/errors.go` con errores tipados
- [x] Middleware de logging: método, path, status, duración (`internal/middleware/logger.go`)

### Tests
- [x] Tests unitarios de `memory/coffee_store.go` (8 tests)
- [x] Tests de integración HTTP para `GET /coffees/{id}` y `GET /coffees` (6 tests)

---

## Fase 2 — Persistencia con PostgreSQL ✅

> Rama: `feature/postgres-store`
> Objetivo: reemplazar el store in-memory con PostgreSQL. El service y los handlers no deben cambiar.

### Infraestructura local
- [x] Crear `docker-compose.yml` con PostgreSQL 16 y Redis 7
- [x] Crear `.env.example` con `DATABASE_URL`, `REDIS_URL`, `PORT`
- [x] Crear `Makefile` con targets: `run`, `test`, `migrate-up`, `migrate-down`, `seed`, `infra-up`, `infra-down`

### Conexión a DB
- [x] Crear `internal/db/connect.go` con pool configurado
- [x] `MaxOpenConns=25`, `MaxIdleConns=5`, `ConnMaxLifetime=5min`
- [x] Health check de la conexión al arrancar el servidor (Ping)

### Migraciones
- [x] Agregar `golang-migrate` v4.17.1 + `lib/pq` como dependencias
- [x] `001_create_producers.up/down.sql`
- [x] `002_create_farms.up/down.sql`
- [x] `003_create_coffees.up/down.sql` (columna generada `available`, trigger `updated_at`)
- [x] `migrations/seed.sql` con los 5 cafés colombianos (IDs consistentes con in-memory)
- [x] `internal/db/migrate.go` para correr migraciones programáticamente al arrancar

### Store PostgreSQL
- [x] Implementar `store/postgres/coffee_store.go`
- [x] `GetByID`: query con JOIN a `farms` y `producers`, `COALESCE` para NULLs
- [x] `List`: query con filtros dinámicos y `$n` params, paginación `LIMIT/OFFSET`
- [x] Manejo de `sql.ErrNoRows` → `ErrNotFound`
- [x] Scanner de `tasting_notes` JSONB → `[]string`

### main.go
- [x] Fallback automático: sin `DATABASE_URL` → in-memory, con `DATABASE_URL` → PostgreSQL + migraciones

### Tests
- [x] Tests de integración con build tag `integration` (`//go:build integration`)
- [x] Se ejecutan con `DATABASE_URL` + `go test -tags integration`
- [x] Excluidos del `go test ./...` normal (sin DB)
- [x] Fase 1: 14 tests unitarios siguen pasando sin cambios

---

## Fase 3 — Usuarios y autenticación ✅

> Rama: `feature/users-auth`
> Objetivo: registro, login y JWT. Base para todo lo que requiera identidad (carrito, órdenes, waitlist).

### Dominio y migraciones
- [x] Crear `internal/domain/user.go` con `User` y `Address`
- [x] `domain/errors.go`: agregar `ErrUnauthorized`
- [x] `004_create_users.up/down.sql`
- [x] `005_create_addresses.up/down.sql`

### Auth utilities
- [x] Crear `internal/auth/jwt.go`: `GenerateTokenPair`, `ValidateAccessToken`, `ValidateRefreshToken`, `UserIDFromContext`
- [x] Crear `internal/auth/password.go`: `HashPassword`, `ComparePassword` (bcrypt cost=12)
- [x] Tokens diferenciados por tipo (`access` vs `refresh`) para evitar uso cruzado

### Store y service
- [x] Interface `UserStore` en `internal/store/user_store.go`
- [x] Implementar `store/memory/user_store.go` (para tests, con mutex)
- [x] Implementar `store/postgres/user_store.go`: `Create`, `GetByEmail`, `GetByID`, `Update`, addresses CRUD
- [x] Catch `pq.Error 23505` → `ErrEmailAlreadyExists` en postgres store
- [x] Crear `internal/service/user_service.go`: `Register`, `Login`, `RefreshToken`, `GetProfile`, `UpdateProfile`, addresses CRUD
- [x] `Register`: trim + lowercase email, validación de campos, hash, genera tokens
- [x] `Login`: devuelve `ErrUnauthorized` tanto si el email no existe como si el password es incorrecto (no revela cuál falló)

### Handlers
- [x] `POST /auth/register` → 201 con `{user, tokens}`
- [x] `POST /auth/login` → 200 con `{user, tokens}`
- [x] `POST /auth/refresh` → 200 con nuevo `TokenPair`
- [x] `GET /users/me` (requiere JWT)
- [x] `PUT /users/me` (requiere JWT)
- [x] `POST /users/me/addresses` (requiere JWT)
- [x] `GET /users/me/addresses` (requiere JWT)
- [x] `DELETE /users/me/addresses/{id}` (requiere JWT) → 204

### Middleware y routes
- [x] `internal/middleware/auth.go`: `RequireAuth(secret)` extrae Bearer, valida, inyecta userID en context
- [x] Refactor `routes/routes.go`: usa struct `Deps` + `requireAuth func(http.Handler) http.Handler`
- [x] `main.go`: wiring completo con fallback in-memory para userStore

### Tests (28 tests total, todos pasando)
- [x] Register: success, duplicate email, missing fields, short password
- [x] Login: success, wrong password, unknown email
- [x] Auth middleware: no token, invalid token
- [x] GetProfile: success con JWT válido
- [x] Refresh: success, invalid token
- [x] AddAddress: success, sin token

---

## Fase 4 — Carrito de compras ✅

> Rama: `feature/cart`
> Objetivo: carrito funcional para invitados (Redis) y usuarios registrados (PostgreSQL) con fusión al login.

### Dominio y migraciones
- [x] Crear `internal/domain/cart.go` con `Cart`, `CartItem`, `CartIdentity`
- [x] `006_create_carts.up/down.sql` (tablas `carts` y `cart_items`)

### Store y service
- [x] Interface `CartStore` en `internal/store/cart_store.go` (`GetItems`, `AddItem`, `SetQuantity`, `RemoveItem`, `Clear`)
- [x] Implementar `store/memory/cart_store.go` (para tests, con mutex)
- [x] Implementar `store/postgres/cart_store.go` (upsert ON CONFLICT, getOrCreateCart)
- [x] Implementar `store/redis/cart_store.go` (JSON en Redis, TTL 30 días)
- [x] Crear `internal/service/cart_service.go`
  - [x] `AddItem` — verifica disponibilidad del café antes de agregar
  - [x] `SetQuantity` — reemplaza cantidad exacta
  - [x] `RemoveItem`, `Clear`, `GetCart`
  - [x] `MergeGuestCart(sessionID, userID)` — toma la cantidad mayor en duplicados
  - [x] `buildCart` — calcula `subtotal_cents` y `total_cents` en memoria

### Middleware y routes
- [x] `middleware.OptionalAuth(secret)` — inyecta userID si JWT válido, sin bloquear si no hay token
- [x] Rutas del carrito usan `optionalAuth`; rutas de usuario usan `requireAuth`
- [x] `routes.Register` actualizado con parámetro `optionalAuth`

### Handlers
- [x] `GET /cart`
- [x] `POST /cart/items`
- [x] `PATCH /cart/items/{coffeeId}`
- [x] `DELETE /cart/items/{coffeeId}`
- [x] `DELETE /cart` → 204

### Lógica de identidad del carrito
- [x] `CartHandler.resolveIdentity`: prioriza userID en context (JWT), cae a `X-Session-ID`
- [x] Sin ninguna identidad → 400 BAD_REQUEST

### Fusión al login
- [x] `AuthHandler` acepta `CartService` opcional
- [x] En `POST /auth/login`: si hay `X-Session-ID`, fusiona carrito invitado al del usuario (best-effort)

### main.go
- [x] Con `DATABASE_URL` + `REDIS_URL` → Redis para invitados, PostgreSQL para usuarios
- [x] Con solo `DATABASE_URL` → in-memory para invitados, PostgreSQL para usuarios
- [x] Sin `DATABASE_URL` → todo in-memory

### Tests (42 tests total, todos pasando)
- [x] GetCart: carrito vacío invitado, sin identidad → 400
- [x] AddItem: éxito, acumula cantidad, café sin stock → 422, café inexistente → 404, quantity ≤ 0 → 400
- [x] SetQuantity: éxito, ítem no en carrito → 404
- [x] RemoveItem: éxito
- [x] ClearCart: vacía y devuelve 204

---

## Fase 5 — Caché con Redis para catálogo ✅

> Rama: `feature/product-detail`
> Objetivo: reducir carga a PostgreSQL en el catálogo. Se hace en esta fase porque Redis ya está presente desde la fase anterior.

### Cache layer
- [x] Interface `CoffeeCache` en `internal/cache/coffee_cache.go`
- [x] Implementar `internal/cache/redis/redis_coffee_cache.go` con `go-redis`
- [x] `GetByID`, `SetByID`, `GetList`, `SetList`, `Invalidate`, `InvalidateAll`
- [x] Implementar `internal/cache/memory/memory_coffee_cache.go` (para tests, con mutex)
- [x] `ListKey(filters)` — clave estable derivada de los filtros, en el package `cache`

### Integración en service
- [x] `CoffeeService.WithCache(c)` — inyección opcional de cache post-construcción
- [x] `CoffeeService.GetByID`: cache-aside (miss → store → cachea, TTL 10 min)
- [x] `CoffeeService.List`: cache por key de filtros (miss → store → cachea, TTL 5 min)
- [x] `CoffeeService.InvalidateProduct(id)` — elimina detail + todas las listas cacheadas

### main.go
- [x] `buildRedisClient()` extrae creación del cliente Redis (compartido entre cart store y coffee cache)
- [x] `buildStores(rc)` recibe el cliente ya construido (no crea uno nuevo internamente)
- [x] Si `REDIS_URL` está presente → habilita cache en `coffeeService`

### Tests (46 tests total, todos pasando)
- [x] `TestCoffeeService_GetByID_cacheMissThenHit` — primera llamada va al store, segunda va al cache
- [x] `TestCoffeeService_List_cacheMissThenHit` — misma lógica para listado
- [x] `TestCoffeeService_InvalidateProduct_forcesStoreMiss` — tras invalidar, vuelve a pegar al store
- [x] `TestCoffeeService_noCache_stillWorks` — sin cache configurado, el service funciona igual

---

## Fase 6 — Checkout y órdenes ✅

> Rama: `feature/checkout-orders`
> Objetivo: flujo de compra completo con desconteo atómico de stock y ciclo de vida de órdenes.

### Dominio y migraciones
- [x] Crear `internal/domain/order.go` con `Order`, `OrderItem`, `OrderStatus`, `IsCancellable()`
- [x] `007_create_orders.up/down.sql` (tablas `orders` y `order_items`, snapshot de dirección y precio)

### Store y service
- [x] Interface `OrderStore` en `internal/store/order_store.go`
- [x] Implementar `store/memory/order_store.go` (para tests, con IDs atómicos)
- [x] Implementar `store/postgres/order_store.go`
  - [x] `Create`: `BEGIN` → `FOR UPDATE` cada café → check/descuenta stock → INSERT orden + ítems → `COMMIT`
  - [x] Si stock insuficiente → rollback + `ErrInsufficientStock`
  - [x] `Cancel`: `BEGIN` → `FOR UPDATE` orden → restaura stock por ítem → UPDATE status → `COMMIT`
- [x] Crear `internal/service/checkout_service.go`
  - [x] `Validate(userID)` — verifica stock de todos los ítems del carrito, devuelve warnings
  - [x] `PlaceOrder(userID, addressID)` — ejecuta la transacción, vacía carrito, publica `order.created`
  - [x] `findAddress` — busca la dirección en el perfil del usuario sin nuevo método en UserStore
- [x] Crear `internal/service/order_service.go`
  - [x] `GetByID(id, userID)` — solo devuelve si la orden pertenece al usuario
  - [x] `ListByUser(userID)` — historial
  - [x] `Cancel(id, userID)` — delega a `orderStore.Cancel`, publica `order.cancelled`

### Handlers
- [x] `GET /checkout/validate`
- [x] `POST /orders`
- [x] `GET /orders`
- [x] `GET /orders/{id}`
- [x] `POST /orders/{id}/cancel`

### Tests
- [x] Test de checkout exitoso: orden creada, ítems correctos
- [x] Test de checkout con carrito vacío: 422
- [x] Test de checkout con stock insuficiente: 422
- [x] Test de checkout sin dirección: 404
- [x] Test de cancelación exitosa: status `cancelled`
- [x] Test de cancelación sin token: 401
- [ ] Test de concurrencia: dos usuarios comprando el último ítem simultáneamente (pendiente)

---

## Fase 7 — Eventos y stock management ✅

> Rama: `feature/stock-events`
> Objetivo: gestión de stock por admin, sistema de eventos en memoria + pg_notify, invalidación de caché basada en eventos.

### Dominio
- [x] `008_create_stock_movements.up/down.sql`
- [x] Crear `internal/domain/stock.go` con `StockMovement`

### Sistema de eventos
- [x] Definir topics y payloads en `internal/event/events.go`
- [x] Interface `Publisher` en `internal/event/publisher.go`
- [x] Implementar `internal/event/bus.go` — bus síncrono en memoria (tests/dev)
- [x] Implementar `internal/event/pg_notify.go` — `PGPublisher` (pg_notify) y `PGListener` (LISTEN + goroutine)
- [x] Consumer: `stock.replenished` → `coffeeService.InvalidateProduct(id)` (wired en main.go)
- [x] Consumer: `stock.depleted` → `coffeeService.InvalidateProduct(id)` (wired en main.go)
- [x] Consumer: `stock.restored` → `waitlistService.NotifyAll(id)` (wired en main.go)

### Admin endpoints de stock
- [x] Crear `internal/service/stock_service.go`
  - [x] `Adjust(coffeeID, delta, note)` — transacción FOR UPDATE, registra movimiento, publica eventos
  - [x] Publica `stock.replenished` + `stock.restored` (si oldStock==0) al sumar
  - [x] Publica `stock.decremented` + `stock.depleted` (si result==0) al restar
- [x] `GET /admin/coffees/{id}/stock`
- [x] `PATCH /admin/coffees/{id}/stock` (body: `{ operation, quantity, note }`)

### Admin endpoints de órdenes
- [x] `GET /admin/orders` (todas, sin filtro por usuario)
- [x] `PATCH /admin/orders/{id}/status`

### Middleware admin
- [x] `internal/middleware/admin.go`: valida `X-Api-Key` contra `ADMIN_API_KEY` env, 401 si falla

### Tests (todos pasando)
- [x] Test de ajuste de stock → `stock.replenished` publicado
- [x] Test de stock que llega a 0 → `stock.depleted` publicado
- [x] Test de stock que sale de 0 → `stock.restored` publicado
- [x] Test de invalidación de caché tras ajuste de stock
- [x] Tests admin handler: GetStock, AdjustStock, ListOrders, UpdateStatus, auth con y sin key

---

## Fase 8 — Waitlist y notificaciones por email ✅

> Rama: `feature/waitlist-notifications`
> Objetivo: usuarios pueden suscribirse a productos sin stock y recibir email cuando vuelvan.

### Dominio y migraciones
- [x] Crear `internal/domain/waitlist.go`
- [x] `009_create_waitlist.up/down.sql` (`UNIQUE(coffee_id, email)`, índice parcial en `notified=false`)

### Store y service
- [x] Interface `WaitlistStore` con `Subscribe`, `GetPending`, `MarkNotified`
- [x] Implementar `store/memory/waitlist_store.go` (suscripción idempotente)
- [x] Implementar `store/postgres/waitlist_store.go` (`ON CONFLICT DO UPDATE` para idempotencia)
- [x] Crear `internal/service/waitlist_service.go`
  - [x] `Subscribe(coffeeID, email, userID?)` — rechaza si el café tiene stock; envía email de confirmación; publica `waitlist.subscribed`
  - [x] `NotifyAll(coffeeID)` — envía emails a todos los pendientes; marca como notificados (idempotente)

### Email sender
- [x] Interface `EmailSender` en `internal/notification/email_sender.go` (nil = no-op)
- [x] Implementación SMTP en `internal/notification/smtp/smtp_sender.go` (`net/smtp`, PlainAuth)
- [x] `buildMailer()` en `main.go` — construye sender si `SMTP_HOST` está definido, nil si no

### Consumers de email (wired en main.go)
- [x] `stock.restored` → `waitlistService.NotifyAll(coffeeID)`
- [ ] `order.created` → email de confirmación (pendiente)
- [ ] `order.shipped` → email con tracking (pendiente)
- [ ] `order.delivered` → email de entrega (pendiente)
- [ ] `order.cancelled` → email de cancelación (pendiente)
- [ ] `user.registered` → email de bienvenida (pendiente)

### Handlers
- [x] `POST /waitlist` (optionalAuth — asocia userID si hay JWT)

### Tests (todos pasando)
- [x] Subscribe exitoso (café sin stock)
- [x] Subscribe rechazado (café con stock) → `ErrInvalidInput`
- [x] Subscribe publica `waitlist.subscribed`
- [x] Subscribe envía email de confirmación
- [x] NotifyAll envía emails y marca como notificados
- [x] NotifyAll idempotente (segunda ejecución no re-envía)
- [x] Subscribe con café inexistente → error
- [x] Subscribe idempotente (mismo email + café retorna misma entrada)
- [x] Subscribe con userID autenticado
- [x] Funciona con nil EmailSender y nil Publisher
- [x] Tests de handler: 201 éxito, 422 en stock, 404 desconocido, 400 faltan campos, 201 con JWT

---

## Fase 9 — Email consumers para eventos de dominio ✅

> Rama: `feature/waitlist-notifications`
> Objetivo: cerrar el ciclo de notificaciones — cada evento de orden y de usuario dispara un email al comprador.

### Correcciones previas (bugs)
- [x] `checkout_service.go` y `order_service.go` publicaban con `map[string]any` + string literal en lugar de `event.OrderPayload` + topic constant
- [x] `AdminHandler` llamaba directamente a `orderStore.UpdateStatus` / `orderStore.ListAll` saltándose el service layer

### Service layer
- [x] Crear `internal/service/notification_service.go`
  - [x] `OnOrderCreated(orderID, userID)` — busca orden y usuario, envía confirmación con resumen de ítems
  - [x] `OnOrderShipped(orderID, userID)` — envía tracking number y dirección de entrega
  - [x] `OnOrderDelivered(orderID, userID)` — avisa que el pedido llegó
  - [x] `OnOrderCancelled(orderID, userID)` — avisa la cancelación con total
  - [x] `OnUserRegistered(email, name)` — email de bienvenida (no necesita lookups)
  - [x] Todos los métodos son no-op si `EmailSender` es nil
- [x] `OrderService.UpdateStatus(id, status, trackingNumber)` — mueve el estado y publica `order.shipped` o `order.delivered` según corresponda
- [x] `OrderService.ListAll(statusFilter, limit, offset)` — delegación limpia para admin
- [x] `UserService.WithPublisher(p)` + publicar `user.registered` en `Register`

### Admin handler refactor
- [x] Eliminar `orderStore` del `AdminHandler` — toda interacción con órdenes pasa por `orderSvc`
- [x] `ListOrders` usa `h.orderSvc.ListAll`
- [x] `UpdateOrderStatus` usa `h.orderSvc.UpdateStatus` (y publica eventos)

### Wiring en main.go
- [x] `userService.WithPublisher(bus)`
- [x] `notificationService = NewNotificationService(mailer, orderStore, userStore)`
- [x] `bus.Subscribe` para: `order.created`, `order.shipped`, `order.delivered`, `order.cancelled`, `user.registered`

### Tests (todos pasando)
- [x] `OnOrderCreated` envía email al usuario correcto
- [x] `OnOrderShipped` envía email con tracking
- [x] `OnOrderDelivered` envía email de entrega
- [x] `OnOrderCancelled` envía email de cancelación
- [x] `OnUserRegistered` envía email de bienvenida
- [x] nil EmailSender → no-op sin panics
- [x] Integración `UserService.Register → bus → OnUserRegistered` (email llega)
- [x] Integración `CheckoutService.PlaceOrder → bus → OnOrderCreated` (email llega)
- [x] Integración `OrderService.UpdateStatus(shipped) → bus → OnOrderShipped` (email llega)
- [x] Integración `OrderService.Cancel → bus → OnOrderCancelled` (email llega)

---

## Fase 10 — Concurrencia, admin CRUD de cafés y CORS ✅

> Rama: `feature/product-detail`
> Objetivo: cerrar los tres ítems pendientes del backlog.

### Test de concurrencia (Fase 6 pendiente)
- [x] `internal/store/postgres/concurrent_checkout_test.go` (`//go:build integration`)
  - [x] Setea stock=1 en un café seeded, lanza 2 goroutines llamando `orderStore.Create` simultáneamente
  - [x] Verifica que exactamente 1 tiene éxito y 1 recibe `ErrInsufficientStock` (garantizado por `FOR UPDATE`)
  - [x] Verifica stock=0 en DB tras el test
  - [x] Cleanup: restaura stock original, elimina user/orders de prueba

### Admin CRUD de cafés
- [x] Extender `CoffeeStore` interface: `Create`, `Update(domain.Coffee)`, `Delete`
- [x] Implementar en `store/memory/coffee_store.go` (refactorizado de slice a map con RWMutex)
- [x] Implementar en `store/postgres/coffee_store.go`:
  - [x] `Create`: transacción INSERT producer → INSERT farm → INSERT coffee → RETURNING id
  - [x] `Update`: UPDATE coffees SET ... WHERE id (preserva farm/producer); llama GetByID para response
  - [x] `Delete`: DELETE FROM coffees WHERE id; ErrNotFound si no existe
- [x] `CoffeeService`: `Create`, `Update(id, input)`, `Delete` con validación e invalidación de cache
- [x] `AdminHandler.WithCoffeeService(coffee)` — inyección opcional post-construcción
- [x] `POST /admin/coffees` → 201 con café creado
- [x] `PUT /admin/coffees/{id}` → 200 (preserva stock/producer/farm)
- [x] `DELETE /admin/coffees/{id}` → 204
- [x] `GET /admin/coffees/{id}/stock` ahora retorna 404 si el café no existe (validación vía coffeeSvc)
- [x] Tests handler: create 201, create 400 faltan campos, update 200, update 404, delete 204, delete 404

### CORS middleware
- [x] `internal/middleware/cors.go`: headers estándar, preflight OPTIONS → 204
- [x] Configurable vía `CORS_ORIGIN` env (default `*`)
- [x] Wired en `main.go` como capa exterior: `middleware.CORS(middleware.Logger(mux))`

### Fixes estructurales
- [x] `countingCoffeeStore` en tests actualizada para implementar la interfaz extendida
- [x] Tests de checkout usan `coffeeStore` directamente en vez de `*CoffeeService` como `store.CoffeeStore`

---

## Checklist de calidad transversal

> Revisar en cada fase antes de mergear a `develop`.

- [ ] No hay lógica de negocio en los handlers (solo parseo de request y formato de response)
- [ ] No hay SQL hardcodeado fuera de `store/postgres/`
- [ ] Errores de dominio tipados en `domain/errors.go`, sin comparar strings de error
- [ ] Toda ruta nueva tiene al menos un test HTTP de integración
- [ ] Passwords nunca aparecen en logs ni en responses
- [ ] Precios siempre en centavos (int), nunca float
- [ ] `go vet ./...` pasa sin errores
- [ ] `go test ./...` pasa sin errores

---

## Progreso general

| Fase | Estado |
|---|---|
| 0 — Fundamentos | ✅ Completa |
| 1 — Catálogo in-memory | ✅ Completa |
| 2 — PostgreSQL | ✅ Completa |
| 3 — Usuarios y auth | ✅ Completa |
| 4 — Carrito | ✅ Completa |
| 5 — Caché Redis catálogo | ✅ Completa |
| 6 — Checkout y órdenes | ✅ Completa |
| 7 — Eventos y stock admin | ✅ Completa |
| 8 — Waitlist y notificaciones | ✅ Completa |
| 9 — Email consumers para eventos de dominio | ✅ Completa |
| 10 — Concurrencia + Admin CRUD cafés + CORS | ✅ Completa |
