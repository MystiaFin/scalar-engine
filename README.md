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

Use PowerShell. You don't need Nix or a C compiler, and the model needs about 8 GB of RAM.

**1. Install the tools**, then close and reopen PowerShell so the new commands are found.

```powershell
winget install Git.Git GoLang.Go Ollama.Ollama
```

**2. Get the project and run it.** The first run creates `.env` for you and stops.

```powershell
git clone https://github.com/MystiaFin/scalar-engine
cd scalar-engine
.\run
```

**3. Fill in `.env`** with your own Gmail address and [App Password](https://support.google.com/accounts/answer/185833). `notepad .env` opens it.

**4. Run it again.** `.\run` is the only command you need from now on, and `.\run test` runs the tests.

To see the data in the app, run [scalar_mobile](https://github.com/MystiaFin/scalar_mobile) next to the server.

## Setup

```bash
# Clone and enter the project
git clone https://github.com/MystiaFin/scalar-engine
cd scalar-engine

# The first run creates .env, then fill it in
./run.sh
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

| Linux and macOS | Windows | What it does |
|---|---|---|
| `./run.sh` | `.\run` | Starts Scalar |
| `./run.sh test` | `.\run test` | Runs the tests |
| `./run.sh pull` | `.\run pull` | Reads older emails |

`./run.sh` uses the Nix dev shell automatically if Ollama isn't installed and Nix is.

### What the run command does

1. Creates `.env` from `.env.example` if it's missing, and stops until you've filled it in.
2. Starts Ollama if it isn't running, and downloads `llama3:8b` if it's missing. The download is about 4.5 GB and only happens once.
3. Starts the server on `:7225`.
4. Reads your emails in the background. An empty database gets the initial pull of `SCALAR_INITIAL_PULL` emails, which leaves them unread and isn't limited to the last 14 days. After that, it runs the normal check on every start and every 30 minutes, and that check can mark emails as read.

If `SCALAR_EMAIL` or `SCALAR_APP_PASSWORD` is empty, or Ollama can't start, it skips reading emails and the API still serves what's saved.

Transactions appear as each email is processed, at `http://localhost:7225/api/transactions`. The server has no web page, so `/` returns 404. The data is read through the API or [scalar_mobile](https://github.com/MystiaFin/scalar_mobile).

To read more older emails later, raise `SCALAR_INITIAL_PULL` and use the pull command. It skips emails already saved.

## Testing

Use `./run.sh test`, or `.\run test` on Windows.

This tests the API against a temporary database. It doesn't need Gmail or Ollama, and it doesn't touch `scalar.db`.

## Nix

A `flake.nix` is provided for a reproducible dev shell on Linux and macOS. Ollama is only available inside this shell:

```bash
nix develop
```

## API

| Method | Path                     | Description                                  |
|--------|--------------------------|----------------------------------------------|
| GET    | `/api/transactions`      | List all transactions, newest first          |
| GET    | `/api/models`            | List the models available in local Ollama    |
| PATCH  | `/api/transactions/:id`  | Update one transaction, or every entry for its merchant when `confirmed` is true  |

**PATCH body:**
```json
{
  "category": "food & drink",
  "is_expense": true,
  "description": "lunch at padang",
  "confirmed": true
}
```

With `"confirmed": true`, this updates every transaction sharing the same merchant name and marks them confirmed. Without it, only this transaction changes. Confirmed transactions are fed back as few-shot examples in future LLM prompts to improve accuracy.

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
│   ├── routes/routes.go            # Route registration
│   ├── routes/routes_test.go       # API tests
│   └── setup/setup.go              # First-run .env, Ollama and model setup
├── run.sh                          # Run and test commands for Linux and macOS
├── run.bat                         # Run and test commands for Windows
├── keywords.json                   # Expense/junk keyword lists
├── credentials.json                # (placeholder)
├── flake.nix                       # Nix dev shell
└── go.mod
```
