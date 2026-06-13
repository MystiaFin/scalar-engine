package email

import (
	"regexp"
	"strconv"
	"strings"
	"time"
)

type ParsedEmail struct {
	Merchant string
	Amount   float64
	Date     string
}

func extractField(body, key string) string {
	lines := strings.Split(body, "\n")
	for _, line := range lines {
		if strings.Contains(strings.ToLower(line), strings.ToLower(key)) {
			parts := strings.SplitN(line, ":", 2)
			if len(parts) == 2 {
				return strings.TrimSpace(parts[1])
			}
		}
	}
	return ""
}

var (
	amountRegex = regexp.MustCompile(`(?i)(?:IDR|Rp\.?)\s*([\d.,]+)`)

	dateRegexes = []*regexp.Regexp{
		regexp.MustCompile(`(\d{2})\s+(Jan|Feb|Mar|Apr|May|Jun|Jul|Aug|Sep|Oct|Nov|Dec)\s+(\d{4})`),
		regexp.MustCompile(`(\d{4})-(\d{2})-(\d{2})`),
		regexp.MustCompile(`(\d{2})/(\d{2})/(\d{4})`),
	}

	monthMap = map[string]string{
		"Jan": "01", "Feb": "02", "Mar": "03", "Apr": "04",
		"May": "05", "Jun": "06", "Jul": "07", "Aug": "08",
		"Sep": "09", "Oct": "10", "Nov": "11", "Dec": "12",
	}
)

func ParseEmail(subject, body string) ParsedEmail {
	return ParsedEmail{
		Merchant: parseMerchant(subject, body),
		Amount:   parseAmount(body),
		Date:     parseDate(body),
	}
}

func parseMerchant(subject, body string) string {
	for _, key := range []string{"payment to", "transfer to", "merchant name", "toko", "kepada"} {
		if v := extractField(body, key); v != "" {
			return v
		}
	}
	return cleanSubject(subject)
}

func cleanSubject(subject string) string {
	prefixes := []string{
		"[E-Receipt]", "[e-receipt]", "Re:", "Fwd:", "FWD:",
		"Terima kasih atas pembayaran Anda -",
		"Konfirmasi Pembayaran",
		"Notifikasi Transaksi",
		"Invoice",
	}
	result := subject
	for _, p := range prefixes {
		result = strings.TrimSpace(strings.TrimPrefix(result, p))
	}
	bracketRegex := regexp.MustCompile(`\[.*?\]\s*`)
	result = bracketRegex.ReplaceAllString(result, "")
	return strings.TrimSpace(result)
}

func parseAmount(body string) float64 {
	match := amountRegex.FindStringSubmatch(body)
	if match == nil {
		return 0
	}

	raw := match[1]

	if strings.Contains(raw, ".") && strings.Contains(raw, ",") {
		raw = strings.ReplaceAll(raw, ".", "")
		raw = strings.ReplaceAll(raw, ",", ".")
	} else if strings.Contains(raw, ".") {
		parts := strings.Split(raw, ".")
		last := parts[len(parts)-1]
		if len(last) == 3 {
			raw = strings.ReplaceAll(raw, ".", "")
		}
	} else if strings.Contains(raw, ",") {
		raw = strings.ReplaceAll(raw, ",", "")
	}

	amount, err := strconv.ParseFloat(raw, 64)
	if err != nil {
		return 0
	}
	return amount
}

func parseDate(body string) string {
	m := dateRegexes[0].FindStringSubmatch(body)
	if m != nil {
		month := monthMap[m[2]]
		return m[3] + "-" + month + "-" + m[1]
	}

	m = dateRegexes[1].FindStringSubmatch(body)
	if m != nil {
		return m[1] + "-" + m[2] + "-" + m[3]
	}

	m = dateRegexes[2].FindStringSubmatch(body)
	if m != nil {
		return m[3] + "-" + m[2] + "-" + m[1]
	}

	return time.Now().Format("2006-01-02")
}
