// cmd/server/main.go
//
// This is ONLY for local development. Vercel does not run this file —
// it uses api/index.go's exported Handler function instead. Run this
// locally with: go run ./cmd/server
package main

import (
	"log"
	"net/http"
	"time"

	"github.com/joho/godotenv"

	// import path adjusted to wherever you place api/index.go's package,
	// e.g. if api/index.go declares `package handler`, import it here.
	handler "github.com/wksn753/kitende-rotary/api"
)

func main() {
	if err := godotenv.Load(); err != nil {
		log.Println("no .env file found, relying on real environment variables")
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/api/", handler.Handler)
	mux.HandleFunc("/api", handler.Handler)

	s := &http.Server{
		Addr:           ":8080",
		Handler:        mux,
		ReadTimeout:    10 * time.Second,
		WriteTimeout:   10 * time.Second,
		MaxHeaderBytes: 1 << 20,
	}

	log.Println("Starting local dev server on port 8080...")
	log.Fatal(s.ListenAndServe())
}