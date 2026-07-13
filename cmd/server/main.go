package main

import (
	"context"
	"embed"
	"flag"
	"fmt"
	"io/fs"
	"log"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"pantry/internal/pantry"
)

//go:embed web/*
var webFiles embed.FS

func main() {
	addr := flag.String("addr", env("PANTRY_ADDR", ":8080"), "listen address")
	data := flag.String("data", env("PANTRY_DATA", "./data"), "data directory")
	flag.Parse()

	if err := os.MkdirAll(*data, 0o750); err != nil {
		log.Fatal(err)
	}
	dbPath := filepath.Join(*data, "pantry.db")
	store, err := pantry.Open(dbPath)
	if err != nil {
		log.Fatal(err)
	}
	defer store.Close()

	web, err := fs.Sub(webFiles, "web")
	if err != nil {
		log.Fatal(err)
	}
	app := pantry.NewServer(store, web)
	server := &http.Server{Addr: *addr, Handler: app, ReadHeaderTimeout: 10 * time.Second}

	go pantry.RunBackups(store, *data, 14)
	go func() {
		log.Printf("Pantry listening on %s", *addr)
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatal(err)
		}
	}()

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, syscall.SIGINT, syscall.SIGTERM)
	<-stop
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := server.Shutdown(ctx); err != nil {
		fmt.Fprintln(os.Stderr, err)
	}
}

func env(key, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return fallback
}
