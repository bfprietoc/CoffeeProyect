package main

import (
	"coffeeproyect/routes"
	"log"
	"net/http"
	"os"
	"time"
)

const (
	defaultPort         = "8080"
	readHeaderTimeoutMs = 5000
)

func main() {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /health", routes.Health)
	port := os.Getenv("PORT")
	if port == "" {
		port = defaultPort
	}
	server := &http.Server{
		Addr:              ":" + port,
		Handler:           mux,
		ReadHeaderTimeout: readHeaderTimeout(),
	}
	log.Printf("listening on %s", server.Addr)
	log.Fatal(server.ListenAndServe())
}
func readHeaderTimeout() time.Duration {
	return time.Duration(readHeaderTimeoutMs) * time.Millisecond
}
