package handlers

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"scalar-rebuild/internal/repository"
)

func GetTransactions(c *gin.Context) {
	transactions, err := repository.GetAll()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, transactions)
}

func UpdateCategory(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid id"})
		return
	}

	var body struct {
		Category    string `json:"category"`
		IsExpense   bool   `json:"is_expense"`
		Description string `json:"description"`
		Confirmed   *bool  `json:"confirmed"`
	}

	if err := c.ShouldBindJSON(&body); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid body"})
		return
	}

	// fetch the transaction first to get its merchant name
	tx, err := repository.GetByID(id)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	if tx == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "transaction not found"})
		return
	}

	if body.Confirmed != nil && *body.Confirmed {
		// bulk update every entry with the same merchant
		affected, err := repository.UpdateByMerchant(tx.Merchant, body.Category, body.IsExpense, body.Description, true)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}

		c.JSON(http.StatusOK, gin.H{
			"message":  "updated",
			"affected": affected,
		})
		return
	}

	// single transaction update
	err = repository.UpdateByID(id, body.Category, body.IsExpense, body.Description)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message":  "updated",
		"affected": 1,
	})
}
