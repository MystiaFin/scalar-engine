package main

import (
	"log"

	"scalar-rebuild/internal/db"
	"scalar-rebuild/internal/email"
	"scalar-rebuild/internal/repository"
	"scalar-rebuild/internal/routes"
	"scalar-rebuild/internal/setup"

	"github.com/gin-gonic/gin"
	"github.com/robfig/cron/v3"
)

// an empty database gets the initial pull, otherwise the normal check
func checkFirstTime() {
	hasData, err := repository.HasTransactions()
	if err != nil {
		log.Printf("failed to read transactions: %v", err)
		return
	}

	if hasData {
		email.Check()
		return
	}

	if _, err := email.Pull(); err != nil {
		log.Printf("failed to run initial pull: %v", err)
	}
}

func main() {
	setup.LoadEnv()

	// init database
	db.Init()

	// without gmail or ollama the api still serves what is saved
	if setup.HasGmail() && setup.PrepareOllama() {
		// start email checker immediately in background
		go checkFirstTime()

		// schedule email check every 30 minutes
		c := cron.New()
		_, err := c.AddFunc("@every 30m", func() {
			log.Println("cron: running email check")
			email.Check()
		})
		if err != nil {
			log.Fatalf("failed to schedule cron: %v", err)
		}
		c.Start()
		defer c.Stop()
	}

	// start gin
	r := gin.Default()
	routes.Register(r)

	log.Println("scalar running on :7225")

	// 2. Serve your CSS/JS
	r.Static("/static", "./frontend-folder")

	// 3. Serve your HTML
	r.GET("/", func(c *gin.Context) {
		c.File("./frontend-folder/index.html")
	})

	r.Run(":7225")
}
