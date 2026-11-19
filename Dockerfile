# syntax=docker/dockerfile:1

# API Go (CoffeeProyect). Binario estático → imagen mínima.
# La config va por env (ver docker-compose.prod.yml del repo web):
# PORT, DATABASE_URL, REDIS_URL, JWT_SECRET, ADMIN_API_KEY.
# El backend auto-migra la DB al arrancar (db.RunMigrations en main.go).

FROM golang:1.25-alpine AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN --mount=type=cache,target=/go/pkg/mod go mod download
COPY . .
RUN --mount=type=cache,target=/go/pkg/mod \
    --mount=type=cache,target=/root/.cache/go-build \
    CGO_ENABLED=0 go build -ldflags="-s -w" -o /out/coffee-api .

FROM alpine:3
WORKDIR /app
RUN adduser -D -u 10001 app
USER app
COPY --from=build /out/coffee-api /usr/local/bin/coffee-api
# El backend auto-migra al arrancar leyendo "migrations/" relativo al CWD.
COPY --chown=app migrations/ /app/migrations/
EXPOSE 8080
ENTRYPOINT ["coffee-api"]
