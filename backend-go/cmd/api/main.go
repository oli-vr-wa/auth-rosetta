package main

import (
	"log/slog"
	"net/http"
	"os"
	"time"

	"authrosetta/internal/api"
)

func main() {
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
