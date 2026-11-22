package main

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"time"

	"authrosetta/internal/api"
	"authrosetta/internal/auth"
	"authrosetta/internal/config"
	"authrosetta/internal/security"
	"authrosetta/internal/store"
)

func main() {
	settings, err := config.Load("../.env")
	if err != nil {
		fmt.Fprintf(os.Stderr, "invalid configuration:\n%v\n", err)
		os.Exit(1)
	}

	slog.SetDefault(slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{
		Level: settings.LogLevel,
	})))
	slog.Info("configuration loaded", "settings", settings.String())

	startCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	pool, err := store.NewPool(startCtx, settings.DatabaseURL())
	cancel()
	if err != nil {
		slog.Error("database unavailable", "err", err)
		os.Exit(1)
	}
	defer pool.Close()
	slog.Info("database connected")

	passwords, err := security.NewPasswords(security.WorkFactor)
	if err != nil {
		fatal("password hashing setup failed", err)
	}
	tokens, err := security.NewTokens(settings.JWTKey, time.Now)
	if err != nil {
		fatal("token setup failed", err)
	}

	service := auth.NewService(
		store.NewUsers(pool), store.NewRevokedTokens(pool),
		passwords, tokens, time.Now,
	)

	srv := &http.Server{
		Addr:              ":5004",
		Handler:           api.NewRouter(service, settings.FrontendOrigin),
		ReadHeaderTimeout: 5 * time.Second,
	}

	slog.Info("listening", "addr", srv.Addr)
	if err := srv.ListenAndServe(); err != nil {
		slog.Error("server stopped", "err", err)
		os.Exit(1)
	}
}

func fatal(message string, err error) {
	slog.Error(message, "err", err)
	os.Exit(1)
}
