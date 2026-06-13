package routes

import (
	"github.com/gin-gonic/gin"
	"scalar-rebuild/internal/handlers"
)

func Register(r *gin.Engine) {
	api := r.Group("/api")
	{
		api.GET("/transactions", handlers.GetTransactions)
		api.PATCH("/transactions/:id", handlers.UpdateCategory)
	}
}
