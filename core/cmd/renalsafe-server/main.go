package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"net"
	"net/http"
	"os"
	"os/signal"
	"renalsafe/core/internal/auth"
	"renalsafe/core/internal/evidence"
	"renalsafe/core/internal/httpapi"
	"renalsafe/core/internal/store"
	"syscall"
	"time"
)

func main() {
	if e := run(); e != nil {
		log.Print("RenalSafe startup failed: ", e)
		os.Exit(1)
	}
}
func run() error {
	addr := flag.String("listen", "127.0.0.1:8080", "local HTTP listen address")
	users := flag.String("users", "../.local/users.json", "role-bound credential hashes")
	seed := flag.String("seed", "../data/knowledge.json", "pending offline seed; empty disables seeding")
	migrate := flag.Bool("migrate", false, "apply explicit database migrations at startup")
	flag.Parse()
	if _, _, e := net.SplitHostPort(*addr); e != nil {
		return e
	}
	dsn := os.Getenv("RENALSAFE_DATABASE_URL")
	if dsn == "" {
		return fmt.Errorf("RENALSAFE_DATABASE_URL required")
	}
	credentials, e := auth.Load(*users)
	if e != nil {
		return fmt.Errorf("credential configuration invalid: %w", e)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	db, e := store.Open(ctx, dsn, false)
	if e != nil {
		return fmt.Errorf("database unavailable")
	}
	defer db.Close()
	if *migrate {
		if e = db.Migrate(ctx); e != nil {
			return e
		}
	}
	if e = db.Provision(ctx, credentials.Actors()); e != nil {
		return e
	}
	if *seed != "" {
		if e = db.Seed(ctx, *seed); e != nil {
			return e
		}
	}
	var embedder evidence.Embedder
	if base := os.Getenv("RENALSAFE_OLLAMA_URL"); base != "" {
		embedder, e = evidence.NewOllama(base, os.Getenv("RENALSAFE_EMBEDDING_MODEL"))
		if e != nil {
			return e
		}
	}
	app := &httpapi.Server{DB: db, Auth: credentials, Embedder: embedder, PDF: evidence.Poppler{}}
	srv := &http.Server{Addr: *addr, Handler: app.Handler(), ReadHeaderTimeout: 5 * time.Second, ReadTimeout: 45 * time.Second, WriteTimeout: 120 * time.Second, IdleTimeout: 60 * time.Second, MaxHeaderBytes: 32 << 10}
	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt, syscall.SIGTERM)
	defer signal.Stop(stop)
	go func() {
		<-stop
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		_ = srv.Shutdown(ctx)
	}()
	log.Printf("RenalSafe local service: http://%s (synthetic patients only)", *addr)
	e = srv.ListenAndServe()
	if e == http.ErrServerClosed {
		return nil
	}
	return e
}
