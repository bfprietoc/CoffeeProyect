# CoffeeProyect — Arquitectura del Sistema

## Visión General

API REST en Go para una tienda de café especializado. El backend gestiona catálogo de productos, carrito de compras, órdenes, stock e inventario, usuarios y notificaciones. La arquitectura está diseñada para ser simple al inicio y escalar de forma incremental sin rewrites.

---

## Stack Tecnológico

| Capa | Tecnología | Justificación |
|---|---|---|
| Lenguaje | Go 1.23+ | Rendimiento, tipado estático, excelente para APIs HTTP |
| HTTP Router | `net/http` estándar | Desde Go 1.22 soporta path params y method routing sin dependencias |
| Base de datos principal | PostgreSQL | ACID para stock e inventario, JSONB para campos flexibles |
| Caché + Sesiones | Redis | Catálogo de productos y carritos de invitados |
| Eventos | PostgreSQL LISTEN/NOTIFY → cola futura | Empieza sin infra extra, migra cuando el volumen lo justifique |
| Migraciones | `golang-migrate` | Versionado de schema, reproducible en cualquier entorno |
| Auth | JWT (access 15min + refresh 7d) | Estándar, stateless, sin dependencias de sesión en DB |
| Contraseñas | `bcrypt` | Estándar de la industria |
| Variables de entorno | `os.Getenv` + `.env` local | Sin dependencias |
| Testing | `testing` estándar + `testify` | Store in-memory como adaptador de tests |

---

## Arquitectura de Capas (Hexagonal)

```
┌──────────────────────────────────────────────────────────────────┐
│                          HTTP Layer                              │
│              internal/handler/  (request / response)             │
│         middleware: auth, logging, CORS, rate-limit              │
└─────────────────────────────┬────────────────────────────────────┘
                              │
┌─────────────────────────────▼────────────────────────────────────┐
│                        Service Layer                             │
│                    internal/service/                             │
│       lógica de negocio, validaciones, orquesta stores           │
│       publica eventos, decide qué cachear                        │
└──────────┬──────────────────────────────────────┬────────────────┘
           │                                      │
┌──────────▼──────────────┐           ┌───────────▼──────────────┐
│      Store (Puerto)      │           │     Event Publisher       │
│   internal/store/        │           │     internal/event/       │
│   CoffeeStore            │           │     (interfaz)            │
│   OrderStore             │           └───────────┬──────────────┘
│   CartStore              │                       │
│   UserStore              │          ┌────────────▼─────────────┐
│   WaitlistStore          │          │  Adaptadores de evento    │
└──────────┬───────────────┘          │  pg_notify / queue        │
           │                          └──────────────────────────┘
┌──────────▼───────────────┐
│    Adaptadores DB/Cache   │
│  store/postgres/          │
│  store/redis/             │
│  store/memory/  (tests)   │
│  cache/redis/             │
│  cache/memory/  (tests)   │
└──────────┬───────────────┘
           │
┌──────────▼──────────────────────────────────────┐
│                  Infraestructura                 │
│   PostgreSQL (fuente de verdad)                  │
│   Redis (caché catálogo + carritos invitados)    │
└─────────────────────────────────────────────────┘
```

### Regla de dependencias
Las capas internas no conocen las externas. El `service` solo habla con interfaces, nunca con `postgres` directamente. Esto permite testear todo con el store in-memory sin levantar infraestructura.

---

## Estructura de Directorios

```
CoffeeProyect/
├── main.go                                → entry point, wiring de dependencias
│
├── internal/
│   ├── domain/
│   │   ├── coffee.go                      → Coffee, Producer, Farm
│   │   ├── order.go                       → Order, OrderItem, OrderStatus, IsCancellable()
│   │   ├── cart.go                        → Cart, CartItem, CartIdentity
│   │   ├── user.go                        → User, Address
│   │   ├── waitlist.go                    → WaitlistEntry
│   │   ├── stock.go                       → StockMovement
│   │   └── errors.go                      → errores de dominio tipados
│   │
│   ├── store/
│   │   ├── coffee_store.go                → interface CoffeeStore
│   │   ├── order_store.go                 → interface OrderStore
│   │   ├── cart_store.go                  → interface CartStore
│   │   ├── user_store.go                  → interface UserStore
│   │   ├── waitlist_store.go              → interface WaitlistStore
│   │   ├── stock_store.go                 → interface StockStore + StockAdjustResult
│   │   ├── postgres/
│   │   │   ├── coffee_store.go
│   │   │   ├── order_store.go             → Create/Cancel con FOR UPDATE + transacción
│   │   │   ├── cart_store.go
│   │   │   ├── user_store.go
│   │   │   ├── waitlist_store.go          → ON CONFLICT idempotente
│   │   │   └── stock_store.go             → Adjust atómico + registro en stock_movements
│   │   ├── redis/
│   │   │   └── cart_store.go              → carrito de invitados (TTL 30d)
│   │   └── memory/
│   │       ├── coffee_store.go            → para tests
│   │       ├── cart_store.go              → para tests
│   │       ├── user_store.go              → para tests
│   │       ├── order_store.go             → para tests (IDs atómicos con sync/atomic)
│   │       ├── waitlist_store.go          → para tests (suscripción idempotente)
│   │       └── stock_store.go             → para tests
│   │
│   ├── service/
│   │   ├── coffee_service.go              → catálogo, filtros, detalle, cache-aside, InvalidateProduct
│   │   ├── cart_service.go                → agregar, quitar, calcular totales
│   │   ├── checkout_service.go            → validar carrito, crear orden, findAddress
│   │   ├── order_service.go               → GetByID (scoped a usuario), ListByUser, Cancel, UpdateStatus, ListAll
│   │   ├── stock_service.go               → Adjust + eventos de stock + invalidación de caché
│   │   ├── user_service.go                → registro, login, perfil; publica user.registered
│   │   ├── waitlist_service.go            → Subscribe (rechaza si en stock) + NotifyAll
│   │   └── notification_service.go        → OnOrder* + OnUserRegistered — consume eventos vía bus
│   │
│   ├── handler/
│   │   ├── health_handler.go
│   │   ├── coffee_handler.go
│   │   ├── cart_handler.go
│   │   ├── checkout_handler.go
│   │   ├── order_handler.go
│   │   ├── user_handler.go
│   │   ├── auth_handler.go
│   │   ├── waitlist_handler.go
│   │   ├── admin_handler.go               → stock admin + orders admin
│   │   └── response.go                    → writeJSON, writeError helpers
│   │
│   ├── middleware/
│   │   ├── auth.go                        → RequireAuth, OptionalAuth (valida JWT)
│   │   ├── admin.go                       → RequireAdmin (valida X-Api-Key)
│   │   └── logger.go                      → logging de requests
│   │
│   ├── event/
│   │   ├── publisher.go                   → interface Publisher
│   │   ├── bus.go                         → Bus síncrono en memoria (Subscribe + Publish)
│   │   ├── pg_notify.go                   → PGPublisher (pg_notify) + PGListener (LISTEN)
│   │   └── events.go                      → topic constants + payload structs
│   │
│   ├── cache/
│   │   ├── coffee_cache.go                → interface CoffeeCache + ListKey helper
│   │   ├── redis/
│   │   │   └── redis_coffee_cache.go      → TTL 10min detalle / 5min lista
│   │   └── memory/
│   │       └── memory_coffee_cache.go     → para tests, sin TTL
│   │
│   ├── notification/
│   │   ├── email_sender.go                → interface EmailSender (nil = no-op)
│   │   └── smtp/
│   │       └── smtp_sender.go             → net/smtp con PlainAuth
│   │
│   ├── auth/
│   │   ├── jwt.go                         → GenerateTokenPair, ValidateAccessToken, UserIDFromContext
│   │   └── password.go                    → HashPassword, ComparePassword (bcrypt cost=12)
│   │
│   └── db/
│       ├── connect.go                     → pool de conexiones PostgreSQL
│       └── migrate.go                     → RunMigrations con golang-migrate
│
├── routes/
│   └── routes.go                          → registro de todas las rutas (Deps struct)
│
├── migrations/
│   ├── 001_create_producers.up/down.sql
│   ├── 002_create_farms.up/down.sql
│   ├── 003_create_coffees.up/down.sql
│   ├── 004_create_users.up/down.sql
│   ├── 005_create_addresses.up/down.sql
│   ├── 006_create_carts.up/down.sql
│   ├── 007_create_orders.up/down.sql      → snapshot de dirección + trigger updated_at
│   ├── 008_create_stock_movements.up/down.sql
│   └── 009_create_waitlist.up/down.sql    → UNIQUE(coffee_id, email) + índice parcial
│
├── docker-compose.yml                     → PostgreSQL + Redis local
├── .env.example
├── Makefile
├── ARCHITECTURE.md
├── PLAN.md
├── EXECUTION.md
├── go.mod
└── go.sum
```

---

## Modelo de Dominio

### Coffee
```go
type Coffee struct {
    ID           string
    Name         string
    Process      string   // washed, natural, honey, anaerobic
    RoastLevel   string   // light, medium, dark
    TastingNotes []string // JSONB en Postgres
    Description  string
    Producer     Producer
    Farm         Farm
    BagSizeGrams int
    PriceCents   int      // centavos, nunca float
    Currency     string   // "COP", "USD"
    Available    bool     // generado: StockBags > 0
    StockBags    int
    CreatedAt    time.Time
    UpdatedAt    time.Time
}
```

### User
```go
type User struct {
    ID           string
    Name         string
    Email        string
    PasswordHash string
    Addresses    []Address
    CreatedAt    time.Time
}

type Address struct {
    ID         string
    UserID     string
    Label      string  // "casa", "oficina"
    Street     string
    City       string
    Department string
    Country    string
    IsDefault  bool
}
```

### Cart
```go
// Cart de invitado vive en Redis (TTL 30 días, clave: cart:guest:{sessionID})
// Cart de usuario registrado vive en PostgreSQL (tablas carts + cart_items)
type Cart struct {
    ID         string
    Items      []CartItem
    TotalCents int
    Currency   string
}

type CartItem struct {
    CoffeeID       string
    CoffeeName     string  // desnormalizado para mostrar sin JOIN
    Quantity       int
    UnitPriceCents int     // precio en el momento de agregar
    BagSizeGrams   int
    SubtotalCents  int     // calculado en memoria por CartService
}

// CartIdentity unifica la identidad de invitado y usuario para el CartService.
// El handler la construye leyendo primero el JWT del context (OptionalAuth)
// y cayendo al header X-Session-ID si no hay token válido.
type CartIdentity struct {
    ID      string
    IsGuest bool
}
```

### Order
```go
type Order struct {
    ID              string
    UserID          string
    Items           []OrderItem
    // Shipping address snapshot (copiado al crear la orden, no depende del perfil)
    ShippingStreet  string
    ShippingCity    string
    ShippingDept    string
    ShippingCountry string
    TotalCents      int
    Currency        string
    Status          OrderStatus
    TrackingNumber  *string
    CreatedAt       time.Time
    UpdatedAt       time.Time
}

type OrderItem struct {
    ID             string
    CoffeeID       string
    CoffeeName     string  // snapshot del nombre al momento de compra
    Quantity       int
    UnitPriceCents int     // snapshot del precio al momento de compra
    SubtotalCents  int     // calculado: Quantity * UnitPriceCents
}

type OrderStatus string
const (
    OrderStatusConfirmed  OrderStatus = "confirmed"
    OrderStatusProcessing OrderStatus = "processing"
    OrderStatusShipped    OrderStatus = "shipped"
    OrderStatusDelivered  OrderStatus = "delivered"
    OrderStatusCancelled  OrderStatus = "cancelled"
)

func (s OrderStatus) IsCancellable() bool {
    return s == OrderStatusConfirmed || s == OrderStatusProcessing
}
```

### WaitlistEntry
```go
type WaitlistEntry struct {
    ID        string
    CoffeeID  string
    UserID    *string  // nil si es invitado
    Email     string
    Notified  bool
    CreatedAt time.Time
}
```

---

## Schema de Base de Datos (PostgreSQL)

```sql
-- Productores
CREATE TABLE producers (
    id   UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name TEXT NOT NULL
);

-- Fincas
CREATE TABLE farms (
    id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name        TEXT NOT NULL,
    country     TEXT NOT NULL,
    region      TEXT NOT NULL,
    producer_id UUID REFERENCES producers(id)
);

-- Catálogo
CREATE TABLE coffees (
    id             UUID    PRIMARY KEY DEFAULT gen_random_uuid(),
    name           TEXT    NOT NULL,
    process        TEXT    NOT NULL,
    roast_level    TEXT    NOT NULL,
    tasting_notes  JSONB   NOT NULL DEFAULT '[]',
    description    TEXT,
    farm_id        UUID    REFERENCES farms(id),
    bag_size_grams INT     NOT NULL,
    price_cents    INT     NOT NULL,
    currency       TEXT    NOT NULL DEFAULT 'COP',
    stock_bags     INT     NOT NULL DEFAULT 0,
    available      BOOLEAN GENERATED ALWAYS AS (stock_bags > 0) STORED,
    created_at     TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at     TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- Usuarios
CREATE TABLE users (
    id            UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name          TEXT NOT NULL,
    email         TEXT NOT NULL UNIQUE,
    password_hash TEXT NOT NULL,
    created_at    TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- Direcciones
CREATE TABLE addresses (
    id         UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id    UUID REFERENCES users(id) ON DELETE CASCADE,
    label      TEXT NOT NULL,
    street     TEXT NOT NULL,
    city       TEXT NOT NULL,
    department TEXT NOT NULL,
    country    TEXT NOT NULL DEFAULT 'CO',
    is_default BOOLEAN NOT NULL DEFAULT false
);

-- Carritos de usuarios registrados
CREATE TABLE carts (
    id         UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id    UUID REFERENCES users(id) ON DELETE CASCADE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE(user_id)  -- un carrito activo por usuario
);

CREATE TABLE cart_items (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    cart_id         UUID REFERENCES carts(id) ON DELETE CASCADE,
    coffee_id       UUID REFERENCES coffees(id),
    quantity        INT NOT NULL CHECK (quantity > 0),
    unit_price_cents INT NOT NULL,
    UNIQUE(cart_id, coffee_id)
);

-- Órdenes
CREATE TABLE orders (
    id                  UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id             UUID REFERENCES users(id),
    shipping_address_id UUID REFERENCES addresses(id),
    total_cents         INT  NOT NULL,
    currency            TEXT NOT NULL,
    status              TEXT NOT NULL DEFAULT 'pending',
    tracking_number     TEXT,
    created_at          TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at          TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE order_items (
    id               UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    order_id         UUID REFERENCES orders(id) ON DELETE CASCADE,
    coffee_id        UUID REFERENCES coffees(id),
    coffee_name      TEXT NOT NULL,  -- snapshot
    quantity         INT  NOT NULL,
    unit_price_cents INT  NOT NULL
);

-- Lista de espera (productos sin stock)
CREATE TABLE waitlist (
    id         UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    coffee_id  UUID REFERENCES coffees(id),
    user_id    UUID REFERENCES users(id),
    email      TEXT NOT NULL,
    notified   BOOLEAN NOT NULL DEFAULT false,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE(coffee_id, email)
);
```

---

## Flujos del Sistema

### Flujo 1 — Descubrimiento del catálogo

```
GET /coffees?roast_level=light&country=Colombia&available=true&page=1&limit=20

  Handler → CoffeeService.List(filters)
    → Intentar Redis: cache:coffees:{hash_de_filtros}
    → Cache HIT  → devolver lista cacheada
    → Cache MISS → PostgreSQL (query con filtros dinámicos)
                 → guardar en Redis (TTL 5 min)
                 → devolver lista
```

Filtros soportados: `roast_level`, `process`, `country`, `available`, `min_price`, `max_price`
Ordenamiento: `name`, `price_asc`, `price_desc`, `newest`
Paginación: `page` + `limit` (máx 50 por página)

---

### Flujo 2 — Detalle de producto

```
GET /coffees/:id

  Handler → CoffeeService.GetByID(id)
    → Intentar Redis: cache:coffee:{id}
    → Cache HIT  → devolver
    → Cache MISS → PostgreSQL JOIN farms + producers
                 → guardar en Redis (TTL 10 min)
                 → devolver

Respuesta incluye:
  - Toda la info del café (descripción, notas, proceso, finca, productor)
  - stock_bags y available
  - Si available=false: el frontend muestra botón "Avisarme cuando llegue"
```

---

### Flujo 3 — Carrito (invitado)

El carrito de un invitado vive completamente en Redis. No requiere autenticación.

```
POST /cart/items  (header: X-Session-ID: {uuid})
  Body: { coffee_id, quantity }

  Handler → CartService.AddItem(sessionID, coffeeID, quantity)
    → Verificar que el café existe y está disponible
    → Leer cart de Redis: cart:guest:{sessionID}
    → Si ya tiene el ítem → sumar cantidad
    → Guardar en Redis (TTL 30 días, se renueva en cada operación)
    → Devolver carrito actualizado con totales calculados

GET /cart  (header: X-Session-ID: {uuid})
  → Leer de Redis
  → Enriquecer con precios actuales (por si cambiaron)
  → Calcular total
  → Devolver carrito

DELETE /cart/items/:coffeeId
  → Leer de Redis → quitar ítem → guardar

PATCH /cart/items/:coffeeId  { quantity: 3 }
  → Leer de Redis → actualizar cantidad → guardar

DELETE /cart
  → Borrar key de Redis (limpiar carrito)
```

**Nota importante:** agregar al carrito NO reserva stock. El stock se descuenta solo cuando se confirma la orden. Esto es intencional — un ítem puede venderse mientras está en el carrito de alguien.

---

### Flujo 4 — Carrito (usuario registrado)

```
POST /auth/login
  → Devuelve JWT

POST /cart/items  (header: Authorization: Bearer {jwt})
  → CartService detecta que hay usuario autenticado
  → Lee carrito de PostgreSQL (tabla carts + cart_items)
  → Guarda cambios en PostgreSQL

Al hacer login con carrito de invitado existente:
POST /auth/login  (header: X-Session-ID: {uuid})
  → Si hay cart:guest:{sessionID} en Redis
    → Fusionar ítems del carrito invitado con el carrito del usuario
    → Ante ítems duplicados → tomar la cantidad mayor
    → Borrar el carrito de invitado de Redis
```

---

### Flujo 5 — Checkout y creación de orden

```
1. Validar carrito
   GET /checkout/validate
     → CartService.Validate()
       → Para cada ítem: verificar que el café existe y tiene stock suficiente
       → Si algún ítem no tiene stock → devolver warning (no bloquear)
       → Devolver resumen: ítems válidos, ítems sin stock, total actualizado

2. Confirmar orden
   POST /orders
   Body: { address_id: "uuid" }  (o address inline si es nueva)

   Handler → CheckoutService.PlaceOrder(userID, cartID, addressID)

     BEGIN TRANSACTION
       → Leer carrito actualizado
       → Para cada ítem:
           SELECT stock_bags FROM coffees WHERE id = ? FOR UPDATE
           → Si stock_bags < quantity → ROLLBACK → ErrInsufficientStock
           → UPDATE coffees SET stock_bags = stock_bags - quantity WHERE id = ?
       → INSERT INTO orders ...
       → INSERT INTO order_items ... (snapshot de nombre y precio)
       → Vaciar carrito (DELETE cart_items)
     COMMIT

     → Publicar evento: order.created
     → Devolver orden con status: "confirmed" y HTTP 201
```

La transacción garantiza que nunca se puede vender más stock del disponible, aunque dos usuarios compren el mismo producto al mismo tiempo.

---

### Flujo 6 — Ciclo de vida de una orden

```
Estados posibles:
  pending → confirmed → processing → shipped → delivered
                     ↘ cancelled → refunded (si ya se cobró)

order.created (creada en checkout)
  → EmailService: "Confirmamos tu pedido #123"
  → Verificar si algún producto quedó en stock 0
    → Si sí: publicar stock.depleted

[Admin] PATCH /admin/orders/:id/status { status: "processing" }
  → Publicar order.processing

[Admin] PATCH /admin/orders/:id/status { status: "shipped", tracking_number: "TK123" }
  → Publicar order.shipped
  → EmailService: "Tu pedido está en camino — tracking: TK123"

[Admin] PATCH /admin/orders/:id/status { status: "delivered" }
  → Publicar order.delivered
  → EmailService: "Tu pedido llegó. ¿Cómo estuvo tu café?"

[Usuario o Admin] POST /orders/:id/cancel
  → Solo válido en estados: pending, confirmed, processing
  BEGIN TRANSACTION
    → UPDATE orders SET status = 'cancelled'
    → Para cada order_item:
        UPDATE coffees SET stock_bags = stock_bags + quantity WHERE id = ?
  COMMIT
  → Publicar order.cancelled
  → EmailService: "Tu orden fue cancelada"
  → Publicar stock.replenished para cada producto restaurado
```

---

### Flujo 7 — Gestión de stock (admin)

```
Ver stock actual:
  GET /admin/coffees/:id/stock
    → Devuelve stock_bags, available, historial de movimientos

Ajuste manual (reposición):
  PATCH /admin/coffees/:id/stock
  Body: { operation: "add", quantity: 20, note: "reposición mensual" }

    → StockService.Adjust(coffeeID, delta, note)
      → UPDATE coffees SET stock_bags = stock_bags + 20
      → INSERT INTO stock_movements (motivo, delta, stock_resultante)
      → Publicar stock.replenished
        → Consumer: invalidar cache Redis del producto
        → Consumer: si era 0 antes → publicar stock.restored
          → Consumer: WaitlistService.NotifyAll(coffeeID)

Reducción manual (merma, descarte):
  PATCH /admin/coffees/:id/stock
  Body: { operation: "subtract", quantity: 2, note: "bolsas dañadas" }
    → Igual pero con delta negativo
    → Si resultado es 0 → publicar stock.depleted
```

Toda operación de stock queda registrada en la tabla `stock_movements` para auditoría.

---

### Flujo 8 — Lista de espera (waitlist)

```
Cuando un producto está sin stock, el frontend muestra "Avisarme cuando llegue".

POST /waitlist
Body: { coffee_id: "uuid", email: "user@email.com" }

  WaitlistService.Subscribe(coffeeID, email, userID?)
    → Verificar que el café existe y está sin stock (si tiene stock, no tiene sentido)
    → INSERT INTO waitlist (coffee_id, email, user_id) ON CONFLICT DO NOTHING
    → EmailService: "Te avisaremos cuando {nombre} esté disponible"
    → Devolver 201

Cuando se dispara stock.restored:
  WaitlistService.NotifyAll(coffeeID)
    → SELECT * FROM waitlist WHERE coffee_id = ? AND notified = false
    → Para cada entrada:
        → EmailService: "¡{nombre} ya está disponible! Compralo antes de que se agote"
        → UPDATE waitlist SET notified = true WHERE id = ?
```

---

### Flujo 9 — Registro y autenticación de usuarios

```
Registro:
  POST /auth/register
  Body: { name, email, password }

    → Validar email único
    → bcrypt.Hash(password, cost=12)
    → INSERT INTO users
    → Generar JWT (access 15min + refresh 7d)
    → Publicar user.registered
      → EmailService: "Bienvenido a CoffeeProyect"
    → Devolver tokens

Login:
  POST /auth/login
  Body: { email, password }

    → SELECT user WHERE email = ?
    → bcrypt.Compare(password, hash)
    → Si X-Session-ID está presente → fusionar carrito invitado
    → Devolver JWT access + refresh token (httpOnly cookie o body, según frontend)

Refresh de token:
  POST /auth/refresh
  Body: { refresh_token }
    → Validar refresh token
    → Emitir nuevo access token

Ver perfil:
  GET /users/me  (requiere JWT)
    → Devolver datos del usuario + direcciones guardadas

Agregar dirección:
  POST /users/me/addresses
  Body: { label, street, city, department, country }
```

---

### Flujo 10 — Notificaciones por email

Todos los emails se disparan desde consumers de eventos, nunca sincrónicamente en los handlers.

| Evento | Email enviado |
|---|---|
| `order.created` | "Confirmamos tu pedido #ID — resumen de ítems y total" |
| `order.shipped` | "Tu pedido está en camino — número de tracking" |
| `order.delivered` | "Tu pedido llegó — invitación a dejar reseña" |
| `order.cancelled` | "Tu orden fue cancelada — instrucciones de reembolso si aplica" |
| `stock.restored` | "¡{café} ya está disponible!" (a todos en la waitlist) |
| `user.registered` | "Bienvenido a CoffeeProyect" |
| `waitlist.subscribed` | "Te avisaremos cuando {café} esté disponible" |

Implementación inicial: directa vía SMTP (biblioteca `net/smtp`).
Implementación futura: servicio externo (Resend, SendGrid) con la misma interface `EmailSender`.

---

## Catálogo Completo de Endpoints

### Públicos (sin auth)
| Método | Path | Descripción |
|---|---|---|
| `GET` | `/health` | Health check |
| `GET` | `/coffees` | Lista de cafés con filtros y paginación |
| `GET` | `/coffees/:id` | Detalle de un café |
| `POST` | `/auth/register` | Registro de usuario |
| `POST` | `/auth/login` | Login |
| `POST` | `/auth/refresh` | Refresh de JWT |
| `GET` | `/cart` | Ver carrito (invitado via X-Session-ID o usuario via JWT) |
| `POST` | `/cart/items` | Agregar ítem al carrito |
| `PATCH` | `/cart/items/:coffeeId` | Actualizar cantidad |
| `DELETE` | `/cart/items/:coffeeId` | Quitar ítem |
| `DELETE` | `/cart` | Vaciar carrito |
| `POST` | `/waitlist` | Suscribirse a aviso de disponibilidad |

### Autenticados (requieren JWT de usuario)
| Método | Path | Descripción |
|---|---|---|
| `GET` | `/users/me` | Perfil del usuario |
| `PUT` | `/users/me` | Actualizar perfil |
| `GET` | `/users/me/addresses` | Listar direcciones |
| `POST` | `/users/me/addresses` | Agregar dirección |
| `DELETE` | `/users/me/addresses/:id` | Eliminar dirección |
| `GET` | `/checkout/validate` | Validar carrito antes de confirmar |
| `POST` | `/orders` | Crear orden desde el carrito |
| `GET` | `/orders` | Historial de órdenes del usuario |
| `GET` | `/orders/:id` | Detalle de orden |
| `POST` | `/orders/:id/cancel` | Cancelar orden (si aplica) |

### Admin (requieren X-Api-Key)
| Método | Path | Descripción |
|---|---|---|
| `POST` | `/admin/coffees` | Crear producto |
| `PUT` | `/admin/coffees/:id` | Actualizar producto |
| `DELETE` | `/admin/coffees/:id` | Eliminar producto |
| `GET` | `/admin/coffees/:id/stock` | Ver stock + historial |
| `PATCH` | `/admin/coffees/:id/stock` | Ajustar stock manualmente |
| `GET` | `/admin/orders` | Listar todas las órdenes |
| `PATCH` | `/admin/orders/:id/status` | Cambiar estado (shipped, delivered, etc.) |

---

## Catálogo Completo de Eventos

```go
// Órdenes
const (
    TopicOrderCreated   = "order.created"
    TopicOrderCancelled = "order.cancelled"
    TopicOrderShipped   = "order.shipped"
    TopicOrderDelivered = "order.delivered"
)

// Stock
const (
    TopicStockDecremented = "stock.decremented"  // descuento por orden
    TopicStockReplenished = "stock.replenished"  // reposición manual
    TopicStockDepleted    = "stock.depleted"     // llegó a 0
    TopicStockRestored    = "stock.restored"     // volvió de 0 a > 0
    TopicStockLow         = "stock.low"          // bajó del umbral de alerta
)

// Usuarios
const (
    TopicUserRegistered = "user.registered"
)

// Waitlist
const (
    TopicWaitlistSubscribed = "waitlist.subscribed"
)

// Payloads principales
type StockPayload struct {
    CoffeeID    string `json:"coffee_id"`
    Delta       int    `json:"delta"`
    OldStock    int    `json:"old_stock"`
    ResultStock int    `json:"result_stock"`
    Note        string `json:"note,omitempty"`
}

type OrderPayload struct {
    OrderID string `json:"order_id"`
    UserID  string `json:"user_id"`
}

type WaitlistPayload struct {
    CoffeeID string `json:"coffee_id"`
    Email    string `json:"email"`
}
```

### Mapa productor → consumidor

| Evento | Producido por | Consumido por |
|---|---|---|
| `order.created` | `CheckoutService` | `NotificationService.OnOrderCreated` (email confirmación) |
| `order.shipped` | `OrderService.UpdateStatus` | `NotificationService.OnOrderShipped` (email con tracking) |
| `order.delivered` | `OrderService.UpdateStatus` | `NotificationService.OnOrderDelivered` (email entregado) |
| `order.cancelled` | `OrderService.Cancel` | `NotificationService.OnOrderCancelled` (email cancelación) |
| `stock.replenished` | `StockService` | `CoffeeService.InvalidateProduct` (invalida Redis) |
| `stock.depleted` | `StockService` | `CoffeeService.InvalidateProduct` (invalida Redis) |
| `stock.restored` | `StockService` | `WaitlistService.NotifyAll` (emails a suscritos) |
| `user.registered` | `UserService.Register` | `NotificationService.OnUserRegistered` (email bienvenida) |
| `waitlist.subscribed` | `WaitlistService.Subscribe` | `WaitlistService` (email confirmación inline) |

---

## Estrategia de Caché (Redis)

| Key | Contenido | TTL | Se invalida cuando |
|---|---|---|---|
| `cache:coffees:{hash}` | Lista paginada de cafés | 5 min | Se modifica cualquier café o stock |
| `cache:coffee:{id}` | Detalle de un café | 10 min | Se modifica ese café o su stock |
| `cart:guest:{sessionID}` | Carrito de invitado | 30 días (rolling) | Se modifica el carrito o hace checkout |

El hash de la key de lista es el MD5 de los query params normalizados, para cachear por combinación de filtros.

---

## Seguridad

- **Endpoints de solo lectura:** públicos, sin auth
- **Endpoints de usuario:** JWT en header `Authorization: Bearer {token}`
  - Access token: 15 minutos
  - Refresh token: 7 días (rotación en cada uso)
- **Endpoints de admin:** API key en header `X-Api-Key`
- **Contraseñas:** bcrypt cost=12, nunca se loggean ni devuelven en respuestas
- **Precios en centavos** (int): sin errores de punto flotante
- **UUIDs** como IDs: evitan enumeración y facilitan distribución futura
- **ReadHeaderTimeout** configurado: protección básica contra Slowloris
- **Stock con `FOR UPDATE`** en transacción de checkout: previene overselling bajo concurrencia
- **Snapshots en order_items**: nombre y precio guardados al momento de la compra, no dependen de cambios futuros al catálogo

---

## Diagrama de Infraestructura

```
Cliente (browser / app móvil)
         │
         ▼
    HTTP API (Go) :8080
    ┌──────────────────────────────────────────────┐
    │  /health  /coffees  /cart  /orders  /auth    │
    │  /checkout  /users  /waitlist  /admin        │
    │                                              │
    │  Middleware: auth JWT, logging, CORS         │
    └───────┬──────────────────────┬───────────────┘
            │                      │
    ┌───────▼──────┐      ┌────────▼─────────┐
    │   Redis      │      │   PostgreSQL      │
    │              │      │                  │
    │ cache:coffee │      │ coffees, farms   │
    │ cache:coffees│      │ producers        │
    │ cart:guest:* │      │ users, addresses │
    └──────────────┘      │ carts, cart_items│
                          │ orders           │
                          │ waitlist         │
                          │ stock_movements  │
                          └────────┬─────────┘
                                   │
                          LISTEN/NOTIFY (eventos)
                                   │
                    ┌──────────────▼──────────────┐
                    │      Event Consumers         │
                    │  (goroutines en el mismo     │
                    │   proceso inicialmente)      │
                    │                              │
                    │ - CacheInvalidator           │
                    │ - EmailSender                │
                    │ - WaitlistNotifier           │
                    │ - StockMonitor               │
                    └─────────────────────────────┘
```
