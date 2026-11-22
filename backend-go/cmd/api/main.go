package main

import (
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"time"

	"authrosetta/internal/api"
	"authrosetta/internal/config"
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

	srv := &http.Server{
		Addr:              ":5004",
		Handler:           api.NewRouter(),
		ReadHeaderTimeout: 5 * time.Second,
	}

	slog.Info("listening", "addr", srv.Addr)
	if err := srv.ListenAndServe(); err != nil {
		slog.Error("server stopped", "err", err)
		os.Exit(1)
	}
}
