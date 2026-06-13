package email

import (
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"log"
	"regexp"
	"strings"
	"time"

	"google.golang.org/api/gmail/v1"
	"scalar-rebuild/internal/models"
	"scalar-rebuild/internal/repository"
)

var htmlTagRe = regexp.MustCompile(`<[^>]+>`)

func stripHTML(raw string) string {
	text := htmlTagRe.ReplaceAllString(raw, " ")
	text = strings.ReplaceAll(text, "&nbsp;", " ")
	text = strings.ReplaceAll(text, "&amp;", "&")
	text = strings.ReplaceAll(text, "&lt;", "<")
	text = strings.ReplaceAll(text, "&gt;", ">")
	text = strings.ReplaceAll(text, "&#39;", "'")
	text = strings.ReplaceAll(text, "&quot;", `"`)
	return strings.Join(strings.Fields(text), " ")
}

func extractFromParts(parts []*gmail.MessagePart) string {
	// First pass: prefer text/plain
	for _, part := range parts {
		if part.MimeType == "text/plain" && part.Body != nil && part.Body.Data != "" {
			data, err := base64.URLEncoding.DecodeString(part.Body.Data)
			if err == nil {
				return string(data)
			}
		}
	}

	// Second pass: recurse into multipart/* containers
	for _, part := range parts {
		if strings.HasPrefix(part.MimeType, "multipart/") {
			if result := extractFromParts(part.Parts); result != "" {
				return result
			}
		}
	}

	// Last resort: fall back to text/html and strip tags
	for _, part := range parts {
		if part.MimeType == "text/html" && part.Body != nil && part.Body.Data != "" {
			data, err := base64.URLEncoding.DecodeString(part.Body.Data)
			if err == nil {
				return stripHTML(string(data))
			}
		}
	}

	return ""
}

func extractBody(payload *gmail.MessagePart) string {
	// Try root body first (simple non-multipart emails)
	if payload.Body != nil && payload.Body.Data != "" {
		data, err := base64.URLEncoding.DecodeString(payload.Body.Data)
		if err == nil {
			return string(data)
		}
	}

	return extractFromParts(payload.Parts)
}

func hashMessageID(messageID string) string {
	h := sha256.Sum256([]byte(messageID))
	return hex.EncodeToString(h[:])
}

func Check() {
	log.Println("starting email check")

	twoWeeksAgo := time.Now().AddDate(0, 0, -14).Unix()
	query := fmt.Sprintf("in:inbox after:%d", twoWeeksAgo)

	srv := NewGmailService()
	r, err := srv.Users.Messages.List("me").Q(query).Do()
	if err != nil {
		log.Printf("unable to retrieve messages: %v", err)
		return
	}

	if len(r.Messages) == 0 {
		log.Println("no new messages found")
		return
	}

	log.Printf("found %d new messages", len(r.Messages))

	for _, msg := range r.Messages {
		fullMsg, err := srv.Users.Messages.Get("me", msg.Id).Format("full").Do()
		if err != nil {
			log.Printf("error getting message %s: %v", msg.Id, err)
			continue
		}

		var subject, from, messageID, date string
		for _, header := range fullMsg.Payload.Headers {
			switch header.Name {
			case "Subject":
				subject = header.Value
			case "From":
				from = header.Value
			case "Message-ID":
				messageID = header.Value
			case "Date":
				date = header.Value
			}
		}

		if !IsExpenseReceipt(subject, from) {
			continue
		}

		// parse email date header, fall back to today if it fails
		parsedDate := parseEmailDate(date)

		emailHash := hashMessageID(messageID)

		exists, err := repository.ExistsByHash(emailHash)

		if err != nil {
			log.Printf("error checking hash: %v", err)
			continue
		}
		if exists {
			log.Printf("already processed, skipping: %s", subject)
			continue
		}

		body := extractBody(fullMsg.Payload)

		log.Printf("potential expense found: %s — sending to ollama", subject)

		result, err := AskOllama(subject, body)
		if err != nil {
			log.Printf("ollama error: %v", err)
			continue
		}

		log.Printf("ollama result: merchant=%s amount=%.0f category=%s is_expense=%v",
			result.Merchant, result.Amount, result.Category, result.IsExpense)

		tx := models.Transaction{
			EmailHash: emailHash,
			Merchant:  result.Merchant,
			Amount:    result.Amount,
			Category:  result.Category,
			IsExpense: result.IsExpense,
			Date:      parsedDate,
		}

		if err := repository.InsertTransaction(tx); err != nil {
			log.Printf("failed to insert transaction: %v", err)
			continue
		}

		log.Printf("saved: %s — %.0f — %s", tx.Merchant, tx.Amount, tx.Category)
	}
}

// parseEmailDate tries common RFC email date formats and falls back to today.
func parseEmailDate(raw string) string {
	formats := []string{
		"Mon, 2 Jan 2006 15:04:05 -0700",
		"Mon, 2 Jan 2006 15:04:05 +0000 (UTC)",
		"2 Jan 2006 15:04:05 -0700",
		"Mon, 2 Jan 2006 15:04:05 MST",
	}

	for _, layout := range formats {
		t, err := time.Parse(layout, strings.TrimSpace(raw))
		if err == nil {
			return t.Format("2006-01-02")
		}
	}

	return time.Now().Format("2006-01-02")
}
