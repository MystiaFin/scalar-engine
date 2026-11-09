# Scalar

Automated personal expense tracker. Scalar connects to your Gmail via IMAP, scans incoming emails for transaction receipts, extracts merchant/amount/category using a local Ollama LLM (Llama 3), and stores everything in a SQLite database. A lightweight Gin API serves the data to a frontend.

## How it works

1. **Email fetching** — Connects to Gmail IMAP and searches for messages from the last 14 days.
2. **Keyword filtering** — Filters out newsletters/promotions, keeping likely receipts (based on `keywords.json`).
3. **LLM extraction** — Sends each receipt email to a local Ollama instance running `llama3:8b`, which returns a structured JSON payload with merchant, amount, category, and expense type.
4. **Storage** — Transactions are saved to a local SQLite database (`scalar.db`). Duplicate emails are skipped via SHA-256 hashing.
5. **API** — `GET /api/transactions` lists all transactions; `PATCH /api/transactions/:id` updates the category for all entries with the same merchant (serves as a correction mechanism — corrections are used as few-shot examples for future LLM prompts).
6. **Scheduling** — An initial email check runs on startup, then repeats every 30 minutes via cron.

## Requirements

- [Go](https://go.dev) 1.26+
- [Ollama](https://ollama.ai) running locally with the `llama3:8b` model
- A Gmail account with an [App Password](https://support.google.com/accounts/answer/185833)

Nix is optional. On Windows, follow [Windows quick start](#windows-quick-start).

## Windows quick start

These steps use PowerShell and don't need Nix or a C compiler. The model needs about 8 GB of RAM.

**1. Install the tools.** Close and reopen PowerShell afterwards, so the new commands are found.

```powershell
winget install Git.Git GoLang.Go Ollama.Ollama
```

Ollama runs in the background on Windows, so you don't need `ollama serve`.

**2. Get the project and the model.**

```powershell
git clone https://github.com/MystiaFin/scalar-engine
cd scalar-engine
Copy-Item .env.example .env
ollama pull llama3:8b
```

**3. Fill in `.env`** with your own Gmail address and [App Password](https://support.google.com/accounts/answer/185833). `notepad .env` opens it.

**4. Pull your first emails**, then start the server. Run both from the repo folder, because `keywords.json` and `scalar.db` are looked up relative to it.

```powershell
go run ./cmd/pull
go run ./cmd/main.go
```

**5. Check the data** from a second PowerShell window. Use `curl.exe`, because `curl` in PowerShell is a different command.

```powershell
curl.exe localhost:7225/api/transactions
```

To see the data in the app, run [scalar_mobile](https://github.com/MystiaFin/scalar_mobile) next to the server.

If `failed` in the pull summary isn't 0, Ollama probably wasn't running or the model wasn't downloaded. Fix that and run the pull again.

## Setup

```bash
# Clone and enter the project
git clone https://github.com/MystiaFin/scalar-engine
cd scalar-engine

# Copy and fill in your environment
cp .env.example .env
```

**.env:**
```
SCALAR_EMAIL=your.email@gmail.com
SCALAR_APP_PASSWORD=your-16-char-app-password
SCALAR_INITIAL_PULL=50
```

`SCALAR_INITIAL_PULL` sets how many of the newest INBOX emails the initial pull reads (default 50). It counts emails before keyword filtering, so fewer transactions than this will be saved.

**keywords.json** — Controls which emails are treated as expense receipts vs. junk. Edit to match the keywords in your own email subjects/senders.

**credentials.json** — (Optional) Reserved for future OAuth use.

## Running

To backfill older emails, run the one-time initial pull first. It needs Ollama running, is not limited to the last 14 days, leaves your messages unread, and skips emails it has already saved if you run it again:

```bash
ollama pull llama3:8b
go run ./cmd/pull
```

Then start the server:

```bash
go run cmd/main.go
```

The server starts on `:7225`:
- **API** — `http://localhost:7225/api/transactions`
- **Ollama** — expects `http://localhost:11434`

The server has no web page, so `/` returns 404. The data is read through the API or [scalar_mobile](https://github.com/MystiaFin/scalar_mobile).

The server checks Gmail when it starts and again every 30 minutes. This check can mark emails as read. If `SCALAR_EMAIL` or `SCALAR_APP_PASSWORD` is empty, the check is skipped and the server still starts.

### Nix

A `flake.nix` is provided for a reproducible dev shell on Linux and macOS. Ollama is only available inside this shell:

```bash
nix develop
```

## API

| Method | Path                     | Description                                  |
|--------|--------------------------|----------------------------------------------|
| GET    | `/api/transactions`      | List all transactions, newest first          |
| GET    | `/api/models`            | List the models available in local Ollama    |
| PATCH  | `/api/transactions/:id`  | Update category, description, and expense flag for all entries by merchant  |

**PATCH body:**
```json
{
  "category": "food & drink",
  "is_expense": true,
  "description": "lunch at padang"
}
```

All fields are optional. This bulk-updates every transaction sharing the same merchant name, sets `confirmed = true`, and updates `description` if provided. Confirmed transactions are fed back as few-shot examples in future LLM prompts to improve accuracy.

## Project structure

```
scalar-rebuild/
├── cmd/main.go                     # Entry point — Gin server, cron scheduling
├── cmd/pull/main.go                # One-time initial email pull
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
