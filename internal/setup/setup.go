package setup

import (
	"encoding/json"
	"log"
	"net/http"
	"os"
	"os/exec"
	"time"

	"scalar-rebuild/internal/email"

	"github.com/joho/godotenv"
)

const (
	ollamaURL        = "http://localhost:11434"
	placeholderEmail = "your.email@gmail.com"
)

// stops on a first run, nothing can read gmail until .env is filled in
func LoadEnv() {
	_, hasEmail := os.LookupEnv("SCALAR_EMAIL")
	_, err := os.Stat(".env")

	if err != nil && !hasEmail {
		createEnv()

		log.Println("created .env, put your gmail address and app password in it, then run this again")
		os.Exit(0)
	}

	if err == nil {
		if err := godotenv.Load(); err != nil {
			log.Fatalf("failed to load .env: %v", err)
		}
	}

	if os.Getenv("SCALAR_EMAIL") == placeholderEmail {
		log.Println(".env still has the example values, put your gmail address and app password in it, then run this again")
		os.Exit(0)
	}
}

func createEnv() {
	example, err := os.ReadFile(".env.example")
	if err != nil {
		log.Fatalf("failed to read .env.example, run this from the repo folder: %v", err)
	}

	if err := os.WriteFile(".env", example, 0600); err != nil {
		log.Fatalf("failed to create .env: %v", err)
	}
}

func HasGmail() bool {
	return os.Getenv("SCALAR_EMAIL") != "" && os.Getenv("SCALAR_APP_PASSWORD") != ""
}

// starts ollama and downloads the model when they are missing
func PrepareOllama() bool {
	if !isOllamaRunning() && !startOllama() {
		return false
	}

	if hasModel() {
		return true
	}

	log.Printf("downloading %s, this only happens once", email.Model)

	cmd := exec.Command("ollama", "pull", email.Model)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Run(); err != nil {
		log.Printf("failed to download %s: %v", email.Model, err)
		return false
	}

	return true
}

func isOllamaRunning() bool {
	resp, err := http.Get(ollamaURL + "/api/tags")
	if err != nil {
		return false
	}
	defer resp.Body.Close()

	return resp.StatusCode == http.StatusOK
}

func startOllama() bool {
	log.Println("starting ollama")

	if err := exec.Command("ollama", "serve").Start(); err != nil {
		log.Printf("failed to start ollama, install it from https://ollama.com or use nix develop: %v", err)
		return false
	}

	for range 20 {
		if isOllamaRunning() {
			return true
		}

		time.Sleep(500 * time.Millisecond)
	}

	log.Println("failed to reach ollama after starting it")
	return false
}

func hasModel() bool {
	resp, err := http.Get(ollamaURL + "/api/tags")
	if err != nil {
		return false
	}
	defer resp.Body.Close()

	var tags struct {
		Models []struct {
			Name string `json:"name"`
		} `json:"models"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&tags); err != nil {
		return false
	}

	for _, model := range tags.Models {
		if model.Name == email.Model {
			return true
		}
	}

	return false
}
