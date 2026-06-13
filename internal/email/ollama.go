package email

import (
	"bytes"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strings"

	"scalar-rebuild/internal/models"
	"scalar-rebuild/internal/repository"
)

type TransactionResult struct {
	Merchant  string  `json:"merchant"`
	Amount    float64 `json:"amount"`
	Category  string  `json:"category"`
	IsExpense bool    `json:"is_expense"`
}

type ollamaRequest struct {
	Model  string          `json:"model"`
	Prompt string          `json:"prompt"`
	Format json.RawMessage `json:"format"`
	Stream bool            `json:"stream"`
}

type ollamaResponse struct {
	Response string `json:"response"`
}

var extractSchema = json.RawMessage(`{
	"type": "object",
	"properties": {
		"merchant": {
			"type": "string"
		},
		"amount": {
			"type": "number"
		},
		"category": {
			"type": "string",
			"enum": ["food & drink", "transport", "utilities", "entertainment", "shopping", "health", "transfer", "other"]
		},
		"is_expense": {
			"type": "boolean"
		}
	},
	"required": ["merchant", "amount", "category", "is_expense"]
}`)

func buildFewShotBlock(examples []models.Transaction) string {
	if len(examples) == 0 {
		return ""
	}

	var sb strings.Builder
	sb.WriteString("Here are past transactions that were confirmed correct — use these as reference:\n")
	for _, ex := range examples {
		sb.WriteString(fmt.Sprintf(
			"  merchant: %s → category: %s, is_expense: %v\n",
			ex.Merchant, ex.Category, ex.IsExpense,
		))
	}
	sb.WriteString("\n")
	return sb.String()
}

func AskOllama(subject, body string) (TransactionResult, error) {
	// pull confirmed user corrections as few-shot examples
	examples, err := repository.GetConfirmedExamples(10)
	if err != nil {
		log.Printf("could not load confirmed examples: %v", err)
		examples = nil
	}

	fewShot := buildFewShotBlock(examples)

	prompt := fmt.Sprintf(`You are parsing a financial transaction email for an Indonesian user.
Extract the merchant name, amount, category, and whether it is an expense.

Category guide (with examples):
- food & drink  : restaurants, cafes, food stalls, warung, nasi, mie, bakso, ayam, kopi, boba, jus, snacks, any food or beverage purchase
- health        : pharmacy (apotek), clinic, hospital, doctor, herbal drinks, jamu, supplement, medical
- transport     : Grab, Gojek, toll, parking, fuel (bensin), KRL, MRT, Transjakarta, bus, taxi
- utilities     : PLN (electricity), PDAM (water), internet, phone credit (pulsa), monthly bills
- entertainment : cinema, streaming (Netflix, Spotify), games, events, hobbies
- shopping      : clothing, electronics, books, household items, marketplace (Tokopedia, Shopee)
- transfer      : bank transfer, virtual account payment, QRIS, e-wallet top-up (GoPay, OVO, Dana)
- other         : anything that clearly does not fit the above

%sSubject: %s
Body: %s

Rules:
- merchant: clean store or service name only — no HTML, no trailing letters like "J", no extra text
- amount: total transaction value as a plain number in IDR, no currency symbol. Use 0 if not found.
- category: pick the single best fit from the list above
- is_expense: true if money is going out, false if money is coming in

Respond only with the JSON object.`, fewShot, subject, body)

	reqBody, err := json.Marshal(ollamaRequest{
		Model:  "qwen3:1.7b",
		Prompt: prompt,
		Format: extractSchema,
		Stream: false,
	})
	if err != nil {
		return TransactionResult{}, fmt.Errorf("marshal: %w", err)
	}

	resp, err := http.Post("http://localhost:11434/api/generate", "application/json", bytes.NewReader(reqBody))
	if err != nil {
		return TransactionResult{}, fmt.Errorf("ollama request: %w", err)
	}
	defer resp.Body.Close()

	var ollamaResp ollamaResponse
	if err := json.NewDecoder(resp.Body).Decode(&ollamaResp); err != nil {
		return TransactionResult{}, fmt.Errorf("decode response: %w", err)
	}

	log.Printf("RAW OLLAMA RESPONSE: '%s'", ollamaResp.Response)

	var result TransactionResult
	if err := json.Unmarshal([]byte(ollamaResp.Response), &result); err != nil {
		return TransactionResult{}, fmt.Errorf("parse json: %w", err)
	}

	return result, nil
}
