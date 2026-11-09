package email

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"log"
	"os"
	"regexp"
	"sort"
	"strings"
	"time"

	"scalar-rebuild/internal/models"
	"scalar-rebuild/internal/repository"

	"github.com/emersion/go-imap"
	"github.com/emersion/go-imap/client"
	"github.com/emersion/go-message/mail"
)

func hashMessageID(messageID string) string {
	h := sha256.Sum256([]byte(messageID))
	return hex.EncodeToString(h[:])
}

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

func FetchCleanEmailText(msg *imap.Message, section *imap.BodySectionName) (string, error) {
	r := msg.GetBody(section)
	if r == nil {
		return "", fmt.Errorf("server didn't return message body")
	}

	mr, err := mail.CreateReader(r)
	if err != nil {
		return "", err
	}

	var htmlFallback string

	for {
		p, err := mr.NextPart()
		if err == io.EOF {
			break
		} else if err != nil {
			log.Println("MIME parse error:", err)
			continue
		}

		switch h := p.Header.(type) {
		case *mail.InlineHeader:
			contentType, _, _ := h.ContentType()
			if contentType == "text/plain" {
				b, _ := io.ReadAll(p.Body)
				return string(b), nil
			} else if contentType == "text/html" {
				b, _ := io.ReadAll(p.Body)
				htmlFallback = string(b)
			}
		}
	}

	if htmlFallback != "" {
		return stripHTML(htmlFallback), nil
	}

	return "", fmt.Errorf("no text body found at all")
}

// what happened to the messages in one run
type Summary struct {
	Fetched      int
	PassedFilter int
	Saved        int
	Skipped      int
	Failed       int
}

func connect(readOnly bool) (*client.Client, *imap.MailboxStatus, error) {
	emailAddress := os.Getenv("SCALAR_EMAIL")
	appPassword := os.Getenv("SCALAR_APP_PASSWORD")

	if emailAddress == "" || appPassword == "" {
		return nil, nil, fmt.Errorf("SCALAR_EMAIL or SCALAR_APP_PASSWORD environment variables not set")
	}

	log.Println("Connecting to imap.gmail.com:993...")
	c, err := client.DialTLS("imap.gmail.com:993", nil)
	if err != nil {
		return nil, nil, fmt.Errorf("IMAP DialTLS error: %w", err)
	}

	if err := c.Login(emailAddress, appPassword); err != nil {
		c.Logout()
		return nil, nil, fmt.Errorf("IMAP Login error: %w", err)
	}

	mbox, err := c.Select("INBOX", readOnly)
	if err != nil {
		c.Logout()
		return nil, nil, fmt.Errorf("IMAP Select INBOX error: %w", err)
	}

	return c, mbox, nil
}

// read-only so old unread messages stay unread
func Pull(limit uint32) (Summary, error) {
	c, mbox, err := connect(true)
	if err != nil {
		return Summary{}, err
	}
	defer c.Logout()

	if mbox.Messages == 0 {
		log.Println("Inbox is empty.")
		return Summary{}, nil
	}

	// sequence numbers follow arrival order, so the highest ones are the newest
	first := uint32(1)
	if mbox.Messages > limit {
		first = mbox.Messages - limit + 1
	}

	seqset := new(imap.SeqSet)
	seqset.AddRange(first, mbox.Messages)

	return processMessages(c, seqset, int(mbox.Messages-first+1)), nil
}

func Check() {
	log.Println("starting IMAP email check")

	c, mbox, err := connect(false)
	if err != nil {
		log.Printf("skipping check: %v", err)
		return
	}
	defer c.Logout()

	if mbox.Messages == 0 {
		log.Println("Inbox is empty.")
		return
	}

	twoWeeksAgo := time.Now().AddDate(0, 0, -14)
	criteria := imap.NewSearchCriteria()
	criteria.Since = twoWeeksAgo

	uids, err := c.Search(criteria)
	if err != nil {
		log.Printf("IMAP Search error: %v", err)
		return
	}

	if len(uids) == 0 {
		log.Println("no new messages found in the last 14 days")
		return
	}

	log.Printf("found %d messages to process", len(uids))

	seqset := new(imap.SeqSet)
	seqset.AddNum(uids...)

	processMessages(c, seqset, len(uids))
}

func processMessages(c *client.Client, seqset *imap.SeqSet, count int) Summary {
	var summary Summary

	messages := make(chan *imap.Message, count)
	go func() {
		if err := c.Fetch(seqset, []imap.FetchItem{imap.FetchEnvelope}, messages); err != nil {
			log.Printf("IMAP Envelope Fetch error: %v", err)
		}
	}()

	var allMsgs []*imap.Message
	for msg := range messages {
		allMsgs = append(allMsgs, msg)
	}

	sort.Slice(allMsgs, func(i, j int) bool {
		return allMsgs[i].Envelope.Date.After(allMsgs[j].Envelope.Date)
	})

	summary.Fetched = len(allMsgs)

	log.Printf("processing %d messages, latest first", len(allMsgs))

	for _, msg := range allMsgs {
		subject := msg.Envelope.Subject
		from := ""
		if len(msg.Envelope.From) > 0 {
			from = msg.Envelope.From[0].Address()
		}
		messageID := msg.Envelope.MessageId
		date := msg.Envelope.Date

		if !IsExpenseReceipt(subject, from) {
			continue
		}

		summary.PassedFilter++

		parsedDate := date.Format("2006-01-02")
		emailHash := hashMessageID(messageID)

		exists, err := repository.ExistsByHash(emailHash)
		if err != nil {
			log.Printf("error checking hash: %v", err)
			summary.Failed++
			continue
		}
		if exists {
			log.Printf("already processed, skipping: %s", subject)
			summary.Skipped++
			continue
		}

		bodySeqset := new(imap.SeqSet)
		bodySeqset.AddNum(msg.SeqNum)
		bodyMessages := make(chan *imap.Message, 1)
		section := &imap.BodySectionName{}
		go func() {
			if err := c.Fetch(bodySeqset, []imap.FetchItem{section.FetchItem()}, bodyMessages); err != nil {
				log.Printf("IMAP Body Fetch error: %v", err)
			}
		}()

		bodyMsg := <-bodyMessages
		if bodyMsg == nil {
			log.Printf("no body returned for %s", subject)
			summary.Failed++
			continue
		}

		body, err := FetchCleanEmailText(bodyMsg, section)
		if err != nil {
			log.Printf("error fetching clean text for %s: %v", subject, err)
			summary.Failed++
			continue
		}

		log.Printf("potential expense found: %s — sending to ollama", subject)

		result, err := AskOllama(subject, body)
		if err != nil {
			log.Printf("ollama error: %v", err)
			summary.Failed++
			continue
		}

		log.Printf("ollama result: merchant=%s amount=%.0f category=%s is_expense=%v",
			result.Merchant, result.Amount, result.Category, result.IsExpense)

		dupExists, err := repository.IsDuplicateTransaction(result.Merchant, result.Amount, parsedDate)
		if err != nil {
			log.Printf("error checking duplicate: %v", err)
		} else if dupExists {
			log.Printf("duplicate skipped (merchant=%s amount=%.0f date=%s): %s",
				result.Merchant, result.Amount, parsedDate, subject)
			summary.Skipped++
			continue
		}

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
			summary.Failed++
			continue
		}

		summary.Saved++
		log.Printf("saved: %s — %.0f — %s", tx.Merchant, tx.Amount, tx.Category)
	}

	return summary
}
