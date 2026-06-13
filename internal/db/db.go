package db

import (
	"database/sql"
	"log"

	_ "modernc.org/sqlite"
)

var DB *sql.DB

func Init() {
	var err error
	DB, err = sql.Open("sqlite", "scalar.db")
	if err != nil {
		log.Fatalf("unable to open database: %v", err)
	}

	if err := DB.Ping(); err != nil {
		log.Fatalf("unable to connect to database: %v", err)
	}

	createTables()
	log.Println("database initialized")
}

func createTables() {
	_, err := DB.Exec(`
		CREATE TABLE IF NOT EXISTS transactions (
			id          INTEGER PRIMARY KEY AUTOINCREMENT,
			email_hash  VARCHAR(64) UNIQUE NOT NULL,
			merchant    VARCHAR(255),
			amount      NUMERIC,
			category    VARCHAR(100),
			is_expense  BOOLEAN,
			date        TIMESTAMP,
			confirmed   BOOLEAN DEFAULT FALSE,
			updated_at  TIMESTAMP DEFAULT CURRENT_TIMESTAMP
		);
	`)
	if err != nil {
		log.Fatalf("unable to create tables: %v", err)
	}
}
