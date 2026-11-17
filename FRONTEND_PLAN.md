# Plan — Frontend (CoffeeProyect Web)

## Contexto

El backend Go (este repo) quedó completo y validado: API REST en `:8080`
con batería E2E de 126 aserciones en verde (modo Postgres + Redis). No existe
frontend. Este plan cubre la creación del front como **repo separado**
(decisión tomada: no monorepo).

**Decisión de despliegue:** todo (front + back + infra) corre self-hosted en
una mini PC siempre disponible — sin pagar infraestructura, el tráfico es
bajo. Ver sección [Despliegue](#despliegue--self-hosted-en-mini-pc).

> Este doc vive en el repo del backend como referencia de proyecto; al crear
> el repo del front se mueve a su raíz.

## Decisiones de stack (recomendadas)

| Decisión | Opción | Por qué |
|---|---|---|
| Framework | **Next.js (App Router) + TypeScript** | SSG/ISR para SEO de las fichas de café, ecosistema grande, self-hosteable con Docker (sin dependencia de serverless) |
| Estilos | **Tailwind CSS** | Rápido de iterar, consistente, sin CSS sprawl |
| Server state | **TanStack Query** | Cacheo, refetch y mutaciones del lado cliente; complementa el cache de Redis del backend |
| Client state | **Zustand** (carrito, sesión) | Mínimo boilerplate para este tamaño de app |
| API client | **fetch wrapper propio + tipos TS a mano** | El backend no expone OpenAPI; los tipos espejan `internal/domain` (ver riesgo 2) |
| Testing | **Vitest + Testing Library**, y Playwright E2E contra el backend real | Mismo criterio de validación que la batería E2E del backend |
| CI/CD | **GitHub Actions** (lint + typecheck + build); deploy manual a la mini PC vía `docker compose` | Ver sección Despliegue |

**Alternativas descartadas:**
- SPA pura (Vite + React): más simple, pero sin SSR/ISR → las fichas de
  producto (lo que más SEO trae en una tienda) no indexan bien.
- Nuxt/Vue: válido, pero menos material y ecosistema que React para este caso.
- Monorepo: complejidad de tooling innecesaria para dos apps independientes.

## Repo del front

- **Nombre sugerido:** `CoffeeProyect-web` (mismo owner, privado como el backend)
- **Estructura:**

```
src/
├── app/                      # App Router (rutas)
│   ├── (public)/             # home, coffees (lista+filtros), coffees/[id], waitlist
│   ├── (auth)/               # login, register
│   ├── account/              # perfil + direcciones (protegido con JWT)
│   ├── cart/                 # carrito (invitado o usuario)
│   ├── checkout/             # validación + confirmación de pedido
│   ├── orders/               # historial + detalle (protegido)
│   └── admin/                # panel admin (protegido por X-Api-Key)
├── lib/
│   ├── api/                  # client con interceptors + endpoints tipados por recurso
│   ├── auth/                 # manejo de tokens, refresh, session guest (X-Session-ID)
│   └── types/                # tipos TS espejo de internal/domain
├── components/               # ui/, catalog/, cart/, orders/, admin/
└── middleware.ts             # protección de rutas (account, orders, admin)
```

## Contrato con el backend (ya probado en E2E)

- **Base URL:** `NEXT_PUBLIC_API_URL` (dev: `http://localhost:8080`;
  producción: `/api` relativo, ruteado por Caddy — ver Despliegue). CORS ya
  está habilitado en el backend (verificado), pero en producción no hace falta.
- **Auth:** JWT access + refresh devueltos en body JSON. Access token en
  memoria, refresh en `localStorage` (MVP — ver riesgo 1).
- **Carrito invitado:** header `X-Session-ID` con un UUID generado
  (`crypto.randomUUID()`) y persistido en `localStorage`.
- **Merge on login:** enviar el `X-Session-ID` en `POST /auth/login` — el
  backend fusiona el carrito invitado al del usuario (flujo probado).
- **Admin:** header `X-Api-Key`; el panel pide la key en su "login" y la
  guarda **solo en memoria** (nunca en localStorage).
- **Errores:** respuesta `{ "error": ..., "code": ... }`. Mapear los codes a
  mensajes de usuario: `NOT_FOUND`, `UNAUTHORIZED`, `EMAIL_CONFLICT`,
  `BAD_REQUEST`, `UNAVAILABLE`, `INTERNAL_ERROR`.
- **Datos frescos:** el backend invalida su cache de Redis correctamente tras
  cada mutación (fix de esta semana), así que el front puede confiar en lo que
  devuelve la API tras mutar (TanStack Query invalida sus queries locales).

## Despliegue — self-hosted en mini PC

**Decisión:** todo (front + back + infra) corre en una mini PC siempre
disponible. Sin pagar infraestructura: el tráfico esperado es bajo.

**Hardware:** ThinkCentre 8va gen, 16 GB RAM, 256 GB SSD. Uso estimado del
stack completo: **~1.5 GB de RAM y 10–30 GB de disco** (web ~250MB, api
~50MB, postgres ~300MB, redis ~50MB, caddy + Docker). Sobrado — incluso
podría correr los stacks de dev y prod a la vez.

**Arquitectura (un solo Docker Compose en la mini PC):**

```
Caddy (reverse proxy + routing, :80/:443)
├── /          → web    (Next.js, `next start`)
└── /api/*     → api    (backend Go, :8080)
postgres  (volume persistente — fuente de verdad)
redis     (volume persistente)
```

- `restart: unless-stopped` en todos los servicios: si la máquina se reinicia
  (corte de luz, update), todo vuelve solo.
- El backend no se expone directo: el front llama a `/api/...` (relativo) y
  Caddy lo rutea → sin CORS en producción.
- `NEXT_PUBLIC_API_URL` relativo (`/api`) en producción,
  `http://localhost:8080` en desarrollo.

**Acceso y TLS — Tailscale (gratis) para empezar:**
- Se instala en la mini PC y en cada dispositivo que necesite acceso
  (laptop, celular).
- La app queda accesible desde cualquier lugar con una URL tipo
  `https://coffeeproyect.tailnet.ts.net` — **HTTPS incluido, sin dominio,
  sin abrir puertos en el router**, funciona incluso con CGNAT.
- Solo entran los miembros del tailnet (vos + quien invites) — apropiado para
  uso propio/equipo chico.

**Si a futuro se necesita acceso público (cambio aditivo):**
Cloudflare Tunnel + dominio (~$10/año): conexión saliente, sin abrir puertos,
URL real con HTTPS automático. La arquitectura Docker no se toca; solo cambia
la puerta por la que entra el tráfico.

**Backups y mantenimiento:**
- `pg_dump` nocturno por cron a un segundo disco (es la única copia de los
  datos — no negociable).
- `docker system prune` en cada deploy (imágenes viejas no llenan el SSD).
- Monitoring: `/health` del backend + healthchecks de Docker; opcional
  uptime-kuma.

**Workflow de deploy (simple, suficiente para este tráfico):**

```bash
# En la mini PC (una vez): instalar Docker + Tailscale
sudo apt install docker.io docker-compose-plugin tailscale

# Cada deploy:
git pull && docker compose -f docker-compose.prod.yml up -d --build
```

El CI (GitHub Actions) solo valida: lint, typecheck, build. El deploy es
manual con el comando de arriba (o un `deploy.sh` que lo envuelva).

## Fases

### Fase 0 — Scaffold (1–2 días)
- `create-next-app` + TS + Tailwind + ESLint/Prettier
- Repo en GitHub + CI (lint, typecheck, build)
- `lib/api`: client con interceptors (inyectar `Authorization`, `X-Session-ID`;
  en 401 → intentar refresh → reintentar la request una vez)
- Tipos TS espejo del domain: `Coffee`, `User`, `Address`, `Cart`,
  `Order`/`OrderItem`, `WaitlistEntry`, `ValidationResult`
- Smoke: `/health` + render del listado de cafés

### Fase 1 — Catálogo público (read-only)
- Home + lista con filtros (`roast_level`, `country`, `process`,
  `available=true`) y paginación (`page`/`limit`, máx 50)
- Ficha de detalle: producer, farm, notas de cata, precio, stock
- Botón "avísame cuando vuelva" (waitlist) en productos sin stock
- States explícitos: loading / error / vacío

### Fase 2 — Carrito invitado + auth
- Carrito: agregar, set quantity (PATCH), quitar; snapshot de nombre/precio
  ya viene del backend
- Register / login / refresh automático
- Merge del carrito invitado al login (el usuario ve su carrito fusionado)

### Fase 3 — Checkout y órdenes
- Selección o creación de dirección → `GET /checkout/validate` (muestra stock
  disponible por ítem y total) → `POST /orders`
- Historial + detalle; cancelación solo en `confirmed`/`processing`
  (el backend devuelve 422 si no es cancelable)

### Fase 4 — Cuenta
- Perfil (nombre) + CRUD de direcciones con `is_default`

### Fase 5 — Panel admin
- "Login" con API key (memoria)
- CRUD de cafés, ajuste de stock con movimientos, gestión de órdenes
  (estado + tracking number)

### Fase 6 — Pulido y deploy self-hosted
- Responsive, metadata/OG por café (SEO), accesibilidad básica
- Playwright E2E contra el backend real (subconjunto de la batería de 126
  aserciones: catálogo, carrito invitado, merge, checkout, cancel)
- Dockerfile multi-stage de Next.js (build → `next start`)
- `docker-compose.prod.yml` en la mini PC (web + api + postgres + redis +
  caddy) — ver sección Despliegue
- Tailscale en la mini PC + dispositivos; primer acceso real vía URL del
  tailnet
- Cron de `pg_dump` + `docker system prune` en el script de deploy

## Riesgos y notas

1. **Tokens en `localStorage`**: un XSS expondría el refresh token.
   Aceptable para MVP; mejora futura: cookies `httpOnly` (requiere cambio en
   el backend para emitir cookies + endpoint de logout).
2. **Sin OpenAPI en el backend**: los tipos TS se mantienen a mano y pueden
   desincronizarse. Opcional (follow-up en el repo del backend): generar spec
   OpenAPI y derivar los tipos.
3. **Admin en el mismo repo**: la API key viaja desde el browser a los
   endpoints `/admin/*`. Para producción seria, el panel debería ir detrás de
   un BFF o con auth propia. Para MVP interno está bien.
4. **Carreras de stock**: el backend garantiza atomicidad (FOR UPDATE,
   verificado con test de concurrencia); el front solo debe tratar el 422
   "out of stock" de checkout como estado válido y mostrarlo.
5. **Punto único de falla (la mini PC)**: si la máquina muere, todo cae.
   Mitigaciones: `restart: unless-stopped`, backups nocturnos de Postgres a
   un segundo disco, y el código vive en GitHub (reconstruir en otra máquina
   = clonar + `docker compose up`).
6. **SSD de 256 GB**: las imágenes Docker acumulan espacio. El
   `docker system prune` en cada deploy mantiene el uso en 10–30 GB.

## Criterio de "listo" por fase

Cada fase termina con: CI verde, los flujos de la fase cubiertos por tests
(componentes + al menos un E2E manual documentado), y el flujo correspondiente
de la batería E2E del backend pasando contra el front real.
