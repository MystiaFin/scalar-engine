# Scalar

Automated personal expense tracker. Scalar connects to your Gmail via IMAP, scans incoming emails for transaction receipts, extracts merchant/amount/category using a local Ollama LLM (Qwen 3), and stores everything in a SQLite database. A lightweight Gin API serves the data to a frontend.

## How it works

1. **Email fetching** — Connects to Gmail IMAP and searches for messages from the last 14 days.
2. **Keyword filtering** — Filters out newsletters/promotions, keeping likely receipts (based on `keywords.json`).
3. **LLM extraction** — Sends each receipt email to a local Ollama instance running `qwen3:1.7b`, which returns a structured JSON payload with merchant, amount, category, and expense type.
4. **Storage** — Transactions are saved to a local SQLite database (`scalar.db`). Duplicate emails are skipped via SHA-256 hashing.
5. **API** — `GET /api/transactions` lists all transactions; `PATCH /api/transactions/:id` updates the category for all entries with the same merchant (serves as a correction mechanism — corrections are used as few-shot examples for future LLM prompts).
6. **Scheduling** — An initial email check runs on startup, then repeats every 30 minutes via cron.

## Requirements

- [Go](https://go.dev) 1.26+
- [Ollama](https://ollama.ai) running locally with the `qwen3:1.7b` model
- A Gmail account with an [App Password](https://support.google.com/accounts/answer/185833)

## Setup

```bash
# Clone and enter the project
git clone <repo-url> scalar
cd scalar

# Copy and fill in your environment
cp .env.example .env
```

**.env:**
```
SCALAR_EMAIL=your.email@gmail.com
SCALAR_APP_PASSWORD=your-16-char-app-password
```

**keywords.json** — Controls which emails are treated as expense receipts vs. junk. Edit to match the keywords in your own email subjects/senders.

**credentials.json** — (Optional) Reserved for future OAuth use.

## Running

```bash
go run cmd/main.go
```

The server starts on `:7225`:
- **API** — `http://localhost:7225/api/transactions`
- **Frontend** — `http://localhost:7225/` (served from `frontend-folder/`)
- **Ollama** — expects `http://localhost:11434`

### Nix

A `flake.nix` is provided for a reproducible dev shell:

```bash
nix develop
```

## API

| Method | Path                     | Description                                  |
|--------|--------------------------|----------------------------------------------|
| GET    | `/api/transactions`      | List all transactions, newest first          |
| PATCH  | `/api/transactions/:id`  | Update category for all entries by merchant  |

**PATCH body:**
```json
{
  "category": "food & drink",
  "is_expense": true
}
```

This bulk-updates every transaction sharing the same merchant name and marks them as confirmed. Confirmed transactions are fed back as few-shot examples in future LLM prompts to improve accuracy.

## Project structure

```
scalar-rebuild/
├── cmd/main.go                     # Entry point — Gin server, cron scheduling
├── config/config.go                # Config placeholder
├── internal/
│   ├── db/db.go                    # SQLite init and schema
│   ├── email/
│   │   ├── check.go                # IMAP connection, email scanning
│   │   ├── filter.go               # Keyword-based receipt/junk filtering
│   │   ├── ollama.go               # LLM prompt + JSON extraction
│   │   └── parser.go               # Regex-based fallback parsing
│   ├── handlers/transaction.go     # HTTP handlers
│   ├── models/transaction.go       # Transaction struct
│   ├── repository/transaction.go   # SQL queries
│   └── routes/routes.go            # Route registration
├── keywords.json                   # Expense/junk keyword lists
├── credentials.json                # (placeholder)
├── flake.nix                       # Nix dev shell
└── go.mod
```
