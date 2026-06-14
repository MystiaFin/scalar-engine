package handlers

import (
	"encoding/json"
	"io"
	"net/http"

	"github.com/gin-gonic/gin"
)

func GetAvailableModels(c *gin.Context) {
	resp, err := http.Get("http://localhost:11434/api/tags")
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H {
			"error": "Could not connect to local Ollama. Is it running?",
			"detail": err.Error(),
		})
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		c.JSON(resp.StatusCode, gin.H {
			"error": "Ollama returned an error",
		})
		return
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to read Ollama response"})
		return
	}

	var result map[string]interface{}
	if err := json.Unmarshal(body, &result); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to parse Ollama JSON"})
		return
	}

	c.JSON(http.StatusOK, result)
}
