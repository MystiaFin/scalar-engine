package repository

import (
	"database/sql"
	"log"
	"strings"

	"scalar-rebuild/internal/db"
	"scalar-rebuild/internal/models"
)

func InsertTransaction(tx models.Transaction) error {
	_, err := db.DB.Exec(`
        INSERT OR IGNORE INTO transactions
            (email_hash, merchant, amount, category, is_expense, date, description)
        VALUES
            (?, ?, ?, ?, ?, ?, ?)
    `, tx.EmailHash, tx.Merchant, tx.Amount, tx.Category, tx.IsExpense, tx.Date, tx.Description)
	return err
}

func ExistsByHash(hash string) (bool, error) {
	var count int
	err := db.DB.QueryRow(
		`SELECT COUNT(1) FROM transactions WHERE email_hash = ?`, hash,
	).Scan(&count)
	return count > 0, err
}

func extractWords(s string) []string {
	clean := strings.Map(func(r rune) rune {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') {
			return r
		}
		return ' '
	}, s)
	return strings.Fields(clean)
}

func IsDuplicateTransaction(merchant string, amount float64, date string) (bool, error) {
	rows, err := db.DB.Query(`
		SELECT merchant FROM transactions
		WHERE amount = ? AND date = ?`, amount, date)
	if err != nil {
		return false, err
	}
	defer rows.Close()

	newM := strings.ToLower(merchant)
	newWords := extractWords(newM)

	for rows.Next() {
		var existing string
		if err := rows.Scan(&existing); err != nil {
			continue
		}
		extM := strings.ToLower(existing)

		if strings.Contains(extM, newM) || strings.Contains(newM, extM) {
			return true, nil
		}

		extWords := extractWords(extM)
		for _, w1 := range newWords {
			if len(w1) < 4 { // Ignore short generic words like "PT", "di", "2"
				continue
			}
			for _, w2 := range extWords {
				if w1 == w2 {
					return true, nil
				}
			}
		}
	}
	return false, nil
}

func GetAll() ([]models.Transaction, error) {
	rows, err := db.DB.Query(`
    SELECT id, email_hash, merchant, amount, category, is_expense, date, description, confirmed, updated_at
    FROM transactions ORDER BY date DESC`)

	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var results []models.Transaction
	for rows.Next() {
		var tx models.Transaction
		var isExpense int
		var confirmed int
		if err := rows.Scan(
			&tx.ID, &tx.EmailHash, &tx.Merchant, &tx.Amount,
			&tx.Category, &isExpense, &tx.Date, &tx.Description, &confirmed, &tx.UpdatedAt,
		); err != nil {
			log.Printf("error scanning row: %v", err)
			continue
		}
		tx.IsExpense = isExpense == 1
		tx.Confirmed = confirmed == 1
		results = append(results, tx)
	}

	return results, nil
}

func GetByID(id int64) (*models.Transaction, error) {
	row := db.DB.QueryRow(`
		SELECT id, email_hash, merchant, amount, category, is_expense, date, description, confirmed, updated_at
		FROM transactions WHERE id = ?
	`, id)

	var tx models.Transaction
	var isExpense, confirmed int
	err := row.Scan(
		&tx.ID, &tx.EmailHash, &tx.Merchant, &tx.Amount,
		&tx.Category, &isExpense, &tx.Date, &tx.Description, &confirmed, &tx.UpdatedAt,
	)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}

	tx.IsExpense = isExpense == 1
	tx.Confirmed = confirmed == 1
	return &tx, nil
}

func GetConfirmedExamples(limit int) ([]models.Transaction, error) {
	rows, err := db.DB.Query(`
		SELECT merchant, amount, category, is_expense
		FROM transactions
		WHERE confirmed = TRUE
		ORDER BY updated_at DESC
		LIMIT ?
	`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var results []models.Transaction
	for rows.Next() {
		var tx models.Transaction
		if err := rows.Scan(&tx.Merchant, &tx.Amount, &tx.Category, &tx.IsExpense); err != nil {
			log.Printf("error scanning row: %v", err)
			continue
		}
		results = append(results, tx)
	}

	return results, nil
}

func UpdateByMerchant(merchant, category string, isExpense bool, description string) (int64, error) {
	result, err := db.DB.Exec(`
		UPDATE transactions
		SET category = ?, is_expense = ?, description = ?, confirmed = TRUE, updated_at = CURRENT_TIMESTAMP
		WHERE merchant = ?
	`, category, isExpense, description, merchant)
	if err != nil {
		return 0, err
	}

	return result.RowsAffected()
}
