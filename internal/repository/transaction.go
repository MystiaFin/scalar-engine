package repository

import (
	"database/sql"
	"log"

	"scalar-rebuild/internal/db"
	"scalar-rebuild/internal/models"
)

func InsertTransaction(tx models.Transaction) error {
	_, err := db.DB.Exec(`
		INSERT OR IGNORE INTO transactions
			(email_hash, merchant, amount, category, is_expense, date)
		VALUES
			(?, ?, ?, ?, ?, ?)
	`, tx.EmailHash, tx.Merchant, tx.Amount, tx.Category, tx.IsExpense, tx.Date)

	return err
}

func ExistsByHash(hash string) (bool, error) {
	var count int
	err := db.DB.QueryRow(
		`SELECT COUNT(1) FROM transactions WHERE email_hash = ?`, hash,
	).Scan(&count)
	return count > 0, err
}

func GetAll() ([]models.Transaction, error) {
	rows, err := db.DB.Query(`
		SELECT id, email_hash, merchant, amount, category, is_expense, date, confirmed, updated_at
		FROM transactions
		ORDER BY date DESC
	`)
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
			&tx.Category, &isExpense, &tx.Date, &confirmed, &tx.UpdatedAt,
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
		SELECT id, email_hash, merchant, amount, category, is_expense, date, confirmed, updated_at
		FROM transactions WHERE id = ?
	`, id)

	var tx models.Transaction
	var isExpense, confirmed int
	err := row.Scan(
		&tx.ID, &tx.EmailHash, &tx.Merchant, &tx.Amount,
		&tx.Category, &isExpense, &tx.Date, &confirmed, &tx.UpdatedAt,
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

func UpdateCategoryByMerchant(merchant, category string, isExpense bool) (int64, error) {
	result, err := db.DB.Exec(`
		UPDATE transactions
		SET category = ?, is_expense = ?, confirmed = TRUE, updated_at = CURRENT_TIMESTAMP
		WHERE merchant = ?
	`, category, isExpense, merchant)
	if err != nil {
		return 0, err
	}

	return result.RowsAffected()
}
