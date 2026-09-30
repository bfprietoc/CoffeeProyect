package main

import (
	rediscache "coffeeproyect/internal/cache/redis"
	"coffeeproyect/internal/db"
	"coffeeproyect/internal/event"
	"coffeeproyect/internal/handler"
	"coffeeproyect/internal/middleware"
	"coffeeproyect/internal/notification"
	"coffeeproyect/internal/notification/smtp"
	"coffeeproyect/internal/service"
	"coffeeproyect/internal/store"
	memorystore "coffeeproyect/internal/store/memory"
	pgstore "coffeeproyect/internal/store/postgres"
	redistore "coffeeproyect/internal/store/redis"
	"coffeeproyect/routes"
	"context"
	"log"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	goredis "github.com/redis/go-redis/v9"
)

const (
	defaultPort         = "8080"
	readHeaderTimeoutMs = 5000
)

func main() {
	jwtSecret := jwtSecretFromEnv()
	adminKey := os.Getenv("ADMIN_API_KEY")
	if adminKey == "" {
		log.Println("WARNING: ADMIN_API_KEY not set — admin endpoints are disabled")
	}

	rc := buildRedisClient()
	coffeeStore, userStore, guestCartStore, userCartStore, orderStore, stockStore, waitlistStore := buildStores(rc)

	bus := event.NewBus()

	coffeeService := service.NewCoffeeService(coffeeStore)
	if rc != nil {
		coffeeService = coffeeService.WithCache(rediscache.NewCoffeeCache(rc))
		log.Println("Redis catalog cache enabled")
	}

	// wire cache invalidation from stock events
	bus.Subscribe(event.TopicStockReplenished, func(_ string, p any) {
		if sp, ok := p.(event.StockPayload); ok {
			coffeeService.InvalidateProduct(sp.CoffeeID)
		}
	})
	bus.Subscribe(event.TopicStockDepleted, func(_ string, p any) {
		if sp, ok := p.(event.StockPayload); ok {
			coffeeService.InvalidateProduct(sp.CoffeeID)
		}
	})

	stockService := service.NewStockService(stockStore, coffeeService, bus)
	userService := service.NewUserService(userStore, jwtSecret).WithPublisher(bus)
	cartService := service.NewCartService(guestCartStore, userCartStore, coffeeStore)
	checkoutService := service.NewCheckoutService(userCartStore, coffeeStore, orderStore, userStore).WithPublisher(bus)
	orderService := service.NewOrderService(orderStore).WithPublisher(bus)

	// buildMailer returns *smtp.Sender which may be nil. Wrapping a typed nil in an
	// interface produces a non-nil interface value, causing nil-pointer panics inside
	// the services. Only assign to the interface when the concrete pointer is non-nil.
	var emailSender notification.EmailSender
	if raw := buildMailer(); raw != nil {
		emailSender = raw
	}
	waitlistService := service.NewWaitlistService(waitlistStore, coffeeStore, emailSender, bus)
	notificationService := service.NewNotificationService(emailSender, orderStore, userStore)

	// stock events → cache invalidation + waitlist
	bus.Subscribe(event.TopicStockRestored, func(_ string, p any) {
		if sp, ok := p.(event.StockPayload); ok {
			if err := waitlistService.NotifyAll(sp.CoffeeID); err != nil {
				log.Printf("waitlist notify error for coffee %s: %v", sp.CoffeeID, err)
			}
		}
	})

	// order events → email notifications
	bus.Subscribe(event.TopicOrderCreated, func(_ string, p any) {
		if op, ok := p.(event.OrderPayload); ok {
			if err := notificationService.OnOrderCreated(op.OrderID, op.UserID); err != nil {
				log.Printf("notification error order.created %s: %v", op.OrderID, err)
			}
		}
	})
	bus.Subscribe(event.TopicOrderShipped, func(_ string, p any) {
		if op, ok := p.(event.OrderPayload); ok {
			if err := notificationService.OnOrderShipped(op.OrderID, op.UserID); err != nil {
				log.Printf("notification error order.shipped %s: %v", op.OrderID, err)
			}
		}
	})
	bus.Subscribe(event.TopicOrderDelivered, func(_ string, p any) {
		if op, ok := p.(event.OrderPayload); ok {
			if err := notificationService.OnOrderDelivered(op.OrderID, op.UserID); err != nil {
				log.Printf("notification error order.delivered %s: %v", op.OrderID, err)
			}
		}
	})
	bus.Subscribe(event.TopicOrderCancelled, func(_ string, p any) {
		if op, ok := p.(event.OrderPayload); ok {
			if err := notificationService.OnOrderCancelled(op.OrderID, op.UserID); err != nil {
				log.Printf("notification error order.cancelled %s: %v", op.OrderID, err)
			}
		}
	})
	// user events → email notifications
	bus.Subscribe(event.TopicUserRegistered, func(_ string, p any) {
		if up, ok := p.(event.UserPayload); ok {
			if err := notificationService.OnUserRegistered(up.Email, up.Name); err != nil {
				log.Printf("notification error user.registered: %v", err)
			}
		}
	})

	// start pg_notify listener if DATABASE_URL is set
	if dsn := os.Getenv("DATABASE_URL"); dsn != "" {
		pgListener := event.NewPGListener(dsn)
		pgListener.Start(context.Background())
		log.Println("pg_notify listener started")
	}

	deps := routes.Deps{
		Health:   handler.NewHealthHandler(),
		Coffee:   handler.NewCoffeeHandler(coffeeService),
		Auth:     handler.NewAuthHandler(userService, cartService),
		User:     handler.NewUserHandler(userService),
		Cart:     handler.NewCartHandler(cartService),
		Checkout: handler.NewCheckoutHandler(checkoutService),
		Order:    handler.NewOrderHandler(orderService),
		Admin:    handler.NewAdminHandler(stockService, orderService).WithCoffeeService(coffeeService),
		Waitlist: handler.NewWaitlistHandler(waitlistService),
	}

	mux := http.NewServeMux()
	routes.Register(mux, deps,
		middleware.RequireAuth(jwtSecret),
		middleware.OptionalAuth(jwtSecret),
		middleware.RequireAdmin(adminKey),
	)

	port := os.Getenv("PORT")
	if port == "" {
		port = defaultPort
	}

	server := &http.Server{
		Addr:              ":" + port,
		Handler:           middleware.CORS(middleware.Logger(mux)),
		ReadHeaderTimeout: time.Duration(readHeaderTimeoutMs) * time.Millisecond,
	}

	log.Printf("listening on %s", server.Addr)
	log.Fatal(server.ListenAndServe())
}

func buildRedisClient() *goredis.Client {
	redisURL := os.Getenv("REDIS_URL")
	if redisURL == "" {
		return nil
	}
	addr := strings.TrimPrefix(redisURL, "redis://")
	return redistore.NewClient(addr)
}

func buildStores(rc *goredis.Client) (store.CoffeeStore, store.UserStore, store.CartStore, store.CartStore, store.OrderStore, store.StockStore, store.WaitlistStore) {
	dsn := os.Getenv("DATABASE_URL")

	if dsn == "" {
		log.Println("DATABASE_URL not set — using in-memory store")
		mem := memorystore.NewCartStore()
		return memorystore.NewCoffeeStore(), memorystore.NewUserStore(), mem, mem,
			memorystore.NewOrderStore(), memorystore.NewStockStore(), memorystore.NewWaitlistStore()
	}

	database, err := db.Connect(dsn)
	if err != nil {
		log.Fatalf("connect to database: %v", err)
	}
	if err := db.RunMigrations(database, "migrations"); err != nil {
		log.Fatalf("run migrations: %v", err)
	}
	log.Println("connected to PostgreSQL")

	var guestCart store.CartStore
	if rc != nil {
		guestCart = redistore.NewCartStore(rc)
		log.Println("connected to Redis (guest carts)")
	} else {
		log.Println("REDIS_URL not set — using in-memory guest cart store")
		guestCart = memorystore.NewCartStore()
	}

	return pgstore.NewCoffeeStore(database),
		pgstore.NewUserStore(database),
		guestCart,
		pgstore.NewCartStore(database),
		pgstore.NewOrderStore(database),
		pgstore.NewStockStore(database),
		pgstore.NewWaitlistStore(database)
}

func buildMailer() *smtp.Sender {
	host := os.Getenv("SMTP_HOST")
	if host == "" {
		log.Println("SMTP_HOST not set — email notifications disabled")
		return nil
	}
	port, _ := strconv.Atoi(os.Getenv("SMTP_PORT"))
	if port == 0 {
		port = 587
	}
	return smtp.NewSender(host, port, os.Getenv("SMTP_USER"), os.Getenv("SMTP_PASS"), os.Getenv("SMTP_FROM"))
}

func jwtSecretFromEnv() string {
	secret := os.Getenv("JWT_SECRET")
	if secret == "" {
		secret = "dev-secret-change-in-production"
		log.Println("WARNING: JWT_SECRET not set, using insecure default")
	}
	return secret
}
