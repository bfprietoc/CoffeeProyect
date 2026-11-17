# Plan de fixes — `feature/fix-error-handling`

Plan de correcciones surgido de la campaña de pruebas E2E en modo completo
(PostgreSQL + Redis, sin in-memory). Estado de partida: batería E2E de 126
aserciones con 124 PASS / 2 FAIL, unitarios verdes e integration 7/7 (con un
bug de cleanup detectado a posteriori).

## Resumen

| # | Bug | Severidad | Estado |
|---|-----|-----------|--------|
| 1 | `GET /coffees/{uuid-mal-formado}` → 500 en vez de 4xx | Alta (usuario final) | ✅ Fixed y verificado |
| 2 | Cache stale tras checkout/cancel (stock no se invalida) | Alta (datos incorrectos hasta 10 min) | ✅ Fixed y verificado |
| 3 | Los 500 no dejan rastro en el log | Media (operabilidad) | ✅ Fixed |
| 4 | 500 transitorio en `POST /waitlist` | Baja (no reproducible) | Cubierto indirectamente por #3 |
| 5 | Código muerto en `waitlist_store.go Subscribe` | Baja (mantenibilidad) | ✅ Fixed |
| 6 | `pg_notify` es código muerto en `main.go` | Baja (diseño) | ⏳ Pendiente de decisión (fuera de esta branch) |
| 7 | Test de concurrencia contamina la DB (cleanup inválido) | Media (confiabilidad de tests) | ✅ Fixed y verificado |

---

## Bug 1 — UUID mal formado → 500 INTERNAL_ERROR

**Síntoma:** `GET /coffees/no-existe-123` → `500 {"code":"INTERNAL_ERROR"}`.
Un UUID *válido* inexistente sí devuelve 404 correctamente.

**Causa raíz:** `internal/store/postgres/coffee_store.go` → `GetByID`: el
string crudo llega al parámetro `$1` de una columna `uuid`; Postgres devuelve
SQLSTATE `22P02` (invalid_text_representation) y el error `pq` no se mapea a
`domain.ErrNotFound`, así que el handler lo trata como error interno.

**Fix:** helper compartido `isInvalidUUID(err)` en
`internal/store/postgres/errors.go` (detecta `pq.Error` con código `22P02`) y
mapeo a `domain.ErrNotFound` en los métodos que reciben ID de usuario:

- `coffee_store.go`: `GetByID`, `Update`, `Delete`
- `order_store.go`: `GetByID`, `GetByIDAndUser`, `Cancel`, `UpdateStatus`
- `waitlist_store.go`: `Subscribe` (parámetro `coffee_id`)

**Decisión de diseño:** 404 (no 400) para mantener consistencia con el
comportamiento de UUID válido inexistente y resolver todos los endpoints con
un solo punto. La validación de formato en cada handler se descartó: es más
código y no cubre los IDs que llegan en body (carrito, waitlist).

**Fuera de scope (follow-up):** el store de usuarios/direcciones tiene la
misma clase de problema (`DELETE /users/me/addresses/{id}` con UUID mal
formado). Se documenta aquí para no inflar el diff.

**Verificación:** test integration nuevo
`TestPostgresCoffeeStore_GetByID_invalidUUID` + aserción E2E existente
(`GET /coffees/{uuid mal formado} → 4xx`).

---

## Bug 2 — Cache stale tras checkout y cancelación

**Síntoma:** después de `POST /orders`, `GET /coffees/{id}` sigue mostrando el
stock anterior durante hasta 10 min (TTL del detalle). Lo mismo al cancelar
(stock restaurado en DB, cache sin invalidar).

**Evidencia:** reproducción limpia con `redis-cli MONITOR`:
FLUSHALL → poblar cache (12) → crear orden → DB=11, API=12, cache=12, y el
monitor registró **cero `DEL`** durante la orden.

**Causa raíz:** `orderStore.Create` descuenta stock en transacción sin
publicar ningún evento; `PlaceOrder` solo publica `order.created` cuyo único
consumidor es el de emails. La invalidación de cache solo ocurre en el flujo
admin (`StockService.Adjust` llama `InvalidateProduct` directo + publica
eventos `stock.*`).

**Fix (sigue el patrón existente de `StockService` — llamada directa, no
evento):**

1. `CheckoutService`: nuevo setter `WithCoffeeService(*CoffeeService)`; en
   `PlaceOrder`, tras el éxito de `orderStore.Create`, iterar los ítems y
   llamar `InvalidateProduct(coffeeID)`.
2. `OrderService`: mismo setter; en `Cancel`, iterar los ítems de la orden y
   invalidar.
3. `order_store.go` → `Cancel`: el orden devuelto **no traía ítems** (solo
   id/status/totales). Se extiende la consulta de `toRestore` para traer
   `coffee_name` y `unit_price_cents` y se devuelve `o.Items` completo.
   Bonus: la respuesta de `POST /orders/{id}/cancel` deja de tener
   `"items": null`.
4. `main.go`: cablear `.WithCoffeeService(coffeeService)` en ambos servicios.

**Decisión de diseño:** invalidación síncrona directa (como `StockService`)
en vez de publicar un evento nuevo: no hay consumidores async que lo necesiten
y evita la ventana de carrera del listener. La carrera residual (un GET
concurrente re-pobla entre el commit y el `DEL`) existe ya en el flujo admin y
es aceptable en instancia única.

**Verificación:** aserciones E2E existentes ("stock C1 descontado" y "stock C1
restaurado tras cancelación") que fallaban antes y deben pasar.

---

## Bug 3 — Los 500 no se loguean

**Síntoma:** `writeError` (response.go) escribe la respuesta sin registrar la
causa. El 500 transitorio de waitlist (bug 4) fue imposible de diagnosticar
porque el log solo tiene la línea de acceso.

**Fix:** nueva helper `writeInternalError(w, err)` en `response.go` que hace
`log.Printf("internal error: %v", err)` y escribe el 500 estándar. Se
reemplazan los 29 call sites de
`writeError(w, http.StatusInternalServerError, "internal error", "INTERNAL_ERROR")`
en los 8 handlers por `writeInternalError(w, err)`.

**Verificación:** `go build` + `go vet`; el log del servidor muestra la causa
en cualquier 500 futuro.

---

## Bug 4 — 500 transitorio en `POST /waitlist`

No reproducible (el endpoint funciona hoy; la SQL verificada vía psql). Se
deja documentado: con el fix #3, si vuelve a pasar, la causa queda en el log.

---

## Bug 5 — Código muerto en `waitlist_store.go Subscribe`

**Síntoma:** bloque
```go
var pqErr *pq.Error
if pqErr != nil { // keep compiler happy
    _ = pqErr
}
```
que nunca se evalúa (la variable siempre es nil).

**Fix:** eliminar el bloque; con el fix #1, `Subscribe` además mapea UUID mal
formado a `ErrNotFound` (y se retira el import de `pq`, que pasa a usarse solo
en `errors.go`).

---

## Bug 6 — `pg_notify` muerto en `main.go` ⏳ PENDIENTE DE DECISIÓN

**Síntoma:** `main.go` arranca un `PGListener` ("pg_notify listener started")
pero **nadie llama a `Subscribe` sobre él**, y `PGPublisher` nunca se
instancia. ARCHITECTURE.md lo documenta como parte del diseño (vía de eventos
para multi-instancia).

**Opciones:**
- **A. Cablearlo de verdad:** `PGPublisher` como publisher (o wrapper del bus)
  + subscripciones del listener a los topics existentes. Habilita varias
  instancias del server compartiendo invalidación de cache y notificaciones.
- **B. Retirarlo:** borrar el listener de `main.go` y corregir
  ARCHITECTURE.md. Menos código, diseño de instancia única explícito.

Se decide después de mergear esta branch (afecta arquitectura, no es un bug
de comportamiento).

---

## Bug 7 — El test de concurrencia contamina la DB

**Síntoma:** `TestConcurrentCheckout_oneSucceeds` pasa pero deja `stock_bags=0`,
la orden de prueba y el usuario de prueba en la DB.

**Causa raíz:** `defer database.Close()` (dentro de la función de test) se
ejecuta **antes** que las funciones `t.Cleanup` (regla de Go: los defers de la
función corren al terminar esta, antes que los cleanups). La restauración de
estado hace `Exec` contra el pool ya cerrado y **todos los errores se
ignoran** (no se chequea el return).

**Fix:** en `concurrent_checkout_test.go`:
1. Reemplazar `defer database.Close()` por `t.Cleanup(func() { database.Close() })`
   registrado **inmediatamente después de conectar** (LIFO → cierra al final).
2. Chequear errores en cada `Exec` del cleanup con `t.Errorf`.

**Verificación:** correr `make test-integration` y confirmar que la DB queda
limpia (stock restaurado, sin órdenes/usuarios de prueba).

---

## Documentado pero NO se toca en esta branch

- **`ErrCartEmpty` → 422 con code `"BAD_REQUEST"`:** el string del código
  contradice el status, pero es cosmético y cambiarlo modifica el contrato de
  la API. Se deja para una discusión de spec.

## Verificación general

1. `go build ./... && go vet ./...`
2. `make test` (unitarios)
3. `make test-integration` + verificación de DB limpia post-test
4. Reiniciar el server con el código nuevo y correr la batería E2E completa
   (126 aserciones): se esperan 126/126 (las 2 que fallaban eran estos bugs).

## Resultados de verificación (ejecutados)

1. `go build ./... && go vet ./...` → **OK**
2. `make test` → **verde** (handler, service, store/memory)
3. `make test-integration` → **8/8 PASS** (incluye el nuevo
   `TestPostgresCoffeeStore_GetByID_invalidUUID`). Post-test, la DB quedó
   limpia: stock C1 restaurado a 12, sin órdenes ni usuarios de prueba
   (verifica el fix #7).
4. Server reiniciado con el código nuevo + batería E2E completa →
   **126/126 PASS** (antes: 124/2). Cero respuestas 500 en toda la corrida.
