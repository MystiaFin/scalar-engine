package main

import (
	"log"

	"scalar-rebuild/internal/db"
	"scalar-rebuild/internal/email"
	"scalar-rebuild/internal/routes"

	"github.com/gin-gonic/gin"
	"github.com/robfig/cron/v3"
)

func main() {
	// init database
	db.Init()

	// start email checker immediately in background
	go email.Check()

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

	// start gin
	r := gin.Default()
	routes.Register(r)

	log.Println("scalar running on :8080")

	// 2. Serve your CSS/JS
	r.Static("/static", "./frontend-folder")

	// 3. Serve your HTML
	r.GET("/", func(c *gin.Context) {
		c.File("./frontend-folder/index.html")
	})

	r.Run(":8080")
}
