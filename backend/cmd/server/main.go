package main

import (
	"context"
	"errors"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"golang.org/x/crypto/bcrypt"

	"github.com/example/be_jwt_golang/backend/internal/auth"
	"github.com/example/be_jwt_golang/backend/internal/branch"
	"github.com/example/be_jwt_golang/backend/internal/config"
	"github.com/example/be_jwt_golang/backend/internal/db"
	"github.com/example/be_jwt_golang/backend/internal/docs"
	"github.com/example/be_jwt_golang/backend/internal/menu"
	"github.com/example/be_jwt_golang/backend/internal/role"
	"github.com/example/be_jwt_golang/backend/internal/user"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("config: %v", err)
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	pool, err := db.Connect(ctx, cfg.DatabaseURL)
	if err != nil {
		log.Fatalf("db: %v", err)
	}
	defer pool.Close()
	if err := db.Migrate(ctx, pool); err != nil {
		log.Fatalf("migrate: %v", err)
	}
	if cfg.SeedDemoUsers {
		if err := db.SeedDemoUsers(ctx, pool, bcrypt.DefaultCost); err != nil {
			log.Fatalf("seed: %v", err)
		}
	}

	tokens := auth.NewTokenManager(cfg.JWTSecret, cfg.JWTTTL)
	handler := auth.NewHandler(user.NewPostgresRepository(pool), tokens, bcrypt.DefaultCost)

	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte("ok"))
	})
	handler.Routes(mux)
	menuRepo := menu.NewPostgresRepository(pool)
	menuHandler := menu.NewHandler(menuRepo)
	menuHandler.Routes(mux, tokens.Middleware)
	role.NewHandler(role.NewPostgresRepository(pool), menuRepo).Routes(mux, tokens.Middleware, menuHandler.RequireMenu)
	branch.NewHandler(branch.NewPostgresRepository(pool)).Routes(mux, tokens.Middleware, menuHandler.RequireMenu)
	if cfg.SwaggerEnabled {
		docs.Register(mux)
		log.Printf("swagger UI enabled at /swagger/")
	}

	srv := &http.Server{
		Addr:              ":" + cfg.Port,
		Handler:           logRequests(mux),
		ReadHeaderTimeout: 5 * time.Second,
	}

	go func() {
		log.Printf("listening on :%s (token TTL %s)", cfg.Port, cfg.JWTTTL)
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Fatalf("server: %v", err)
		}
	}()

	<-ctx.Done()
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	_ = srv.Shutdown(shutdownCtx)
	log.Println("server stopped")
}

func logRequests(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		next.ServeHTTP(w, r)
		log.Printf("%s %s %s", r.Method, r.URL.Path, time.Since(start))
	})
}
