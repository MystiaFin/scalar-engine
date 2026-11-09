package main

import (
	"log"
	"os"
	"strconv"

	"scalar-rebuild/internal/db"
	"scalar-rebuild/internal/email"

	"github.com/joho/godotenv"
)

const defaultInitialPull = 50

func readPullLimit() uint32 {
	raw := os.Getenv("SCALAR_INITIAL_PULL")
	if raw == "" {
		return defaultInitialPull
	}

	limit, err := strconv.ParseUint(raw, 10, 32)
	if err != nil || limit == 0 {
		log.Fatalf("SCALAR_INITIAL_PULL must be a positive number, got %q", raw)
	}

	return uint32(limit)
}

func main() {
	if err := godotenv.Load(); err != nil {
		log.Println("No .env file found, relying on system environment variables")
	}

	limit := readPullLimit()

	db.Init()
	defer db.DB.Close()

	log.Printf("starting initial pull of the newest %d emails", limit)

	summary, err := email.Pull(limit)
	if err != nil {
		log.Fatalf("failed to run initial pull: %v", err)
	}

	log.Printf("initial pull done: fetched=%d passed_filter=%d saved=%d skipped=%d failed=%d",
		summary.Fetched, summary.PassedFilter, summary.Saved, summary.Skipped, summary.Failed)
}
