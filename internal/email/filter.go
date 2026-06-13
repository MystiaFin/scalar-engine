package email

import (
	"encoding/json"
	"log"
	"os"
	"strings"
	"sync"
)

type Keywords struct {
	Expense []string `json:"expense"`
	Junk    []string `json:"junk"`
}

var (
	keywords     Keywords
	keywordsOnce sync.Once
)

func getKeywords() Keywords {
	keywordsOnce.Do(func() {
		b, err := os.ReadFile("keywords.json")
		if err != nil {
			log.Fatalf("unable to read keywords.json: %v", err)
		}
		if err := json.Unmarshal(b, &keywords); err != nil {
			log.Fatalf("unable to parse keywords.json: %v", err)
		}
	})
	return keywords
}

func IsExpenseReceipt(subject, from string) bool {
	kw := getKeywords()

	subject = strings.ToLower(subject)
	from = strings.ToLower(from)

	for _, junk := range kw.Junk {
		if strings.Contains(subject, junk) || strings.Contains(from, junk) {
			return false
		}
	}

	for _, keyword := range kw.Expense {
		if strings.Contains(subject, keyword) {
			return true
		}
	}

	return false
}
