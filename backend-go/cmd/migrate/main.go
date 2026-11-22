package main

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"time"

	_ "github.com/jackc/pgx/v5/stdlib"
	"github.com/pressly/goose/v3"

	"authrosetta/internal/config"
	"authrosetta/migrations"
)

func main() {
	if len(os.Args) != 2 {
		fmt.Fprintf(os.Stderr, "usage: go run ./cmd/migrate <up|down|status>")
		os.Exit(2)
	}
	if err := run(os.Args[1]); err != nil {
		fmt.Fprintf(os.Stderr, "migrate: %v\n", err)
		os.Exit(1)
	}
}

// run executes the given migration command ("up", "down", "status") using the Goose migration provider.
func run(command string) error {
	settings, err := config.Load("../.env")
	if err != nil {
		return fmt.Errorf("invalid configuration:\n%w", err)
	}

	// Open a connection to the database using the provided settings.
	db, err := sql.Open("pgx", settings.DatabaseURL())
	if err != nil {
		return fmt.Errorf("opening database: %w", err)
	}
	defer db.Close()

	// Create a context with a timeout to ensure the migration operations do not hang indefinitely.
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Initialize the Goose migration provider with the database connection and migration files.
	provider, err := goose.NewProvider(goose.DialectPostgres, db, migrations.FS)
	if err != nil {
		return fmt.Errorf("creating migration provider: %w", err)
	}

	// Execute the migration command based on the user's input.
	switch command {
	case "up":
		results, err := provider.Up(ctx)
		if err != nil {
			return err
		}
		if len(results) == 0 {
			fmt.Println("already up to date")
		}
		for _, r := range results {
			fmt.Println(r.String())
		}
	case "down":
		result, err := provider.Down(ctx)
		if err != nil {
			return err
		}
		fmt.Println(result.String())
	case "status":
		statuses, err := provider.Status(ctx)
		if err != nil {
			return err
		}
		for _, s := range statuses {
			fmt.Printf("%-50s %s\n", s.Source.Path, s.State)
		}
	default:
		return fmt.Errorf("unknown command: %s", command)
	}
	return nil
}
