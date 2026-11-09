package main

import (
	"log"

	"scalar-rebuild/internal/db"
	"scalar-rebuild/internal/email"
	"scalar-rebuild/internal/setup"
)

func main() {
	setup.LoadEnv()

	if !setup.PrepareOllama() {
		log.Fatal("failed to prepare ollama")
	}

	db.Init()
	defer db.DB.Close()

	if _, err := email.Pull(); err != nil {
		log.Fatalf("failed to run initial pull: %v", err)
	}
}
