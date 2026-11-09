# Scalar Engine Project Context

This document describes the repository as implemented. It is intended to give maintainers and coding agents enough context to work on the project without first rediscovering its structure and runtime behavior.

## Project Summary

Scalar Engine is a small, single-process personal expense ingestion service written in Go. It connects to one Gmail inbox over IMAP, identifies likely transaction emails, asks a locally running Ollama model to extract transaction data, stores the results in SQLite, and exposes the stored data through a Gin HTTP API.

The complete active pipeline is:

```text
Gmail IMAP
  -> subject/sender keyword filter
  -> MIME text extraction
  -> Ollama structured extraction
  -> duplicate checks
  -> SQLite transactions table
  -> Gin JSON API
```

The service is designed for local, single-user operation. It has no user/account model, authentication, hosted LLM integration, or production deployment configuration.

## Technology

- Language: Go
- Module name: `scalar-rebuild`
- Go directive: `1.26.4`
- HTTP framework: Gin
- Database: SQLite through the pure-Go `modernc.org/sqlite` driver
- Email: Gmail IMAP through `github.com/emersion/go-imap`
- MIME parsing: `github.com/emersion/go-message`
- Scheduling: `github.com/robfig/cron/v3`
- Environment-file loading: `github.com/joho/godotenv`
- LLM runtime: local Ollama HTTP API
- Development environment: optional Nix flake

There are no tests, CI workflows, containers, release scripts, formal database migrations, or tracked frontend assets.

## Repository Layout

```text
.
├── cmd/main.go
├── config/config.go
├── internal/
│   ├── db/db.go
│   ├── email/
│   │   ├── check.go
│   │   ├── filter.go
│   │   ├── ollama.go
│   │   └── parser.go
│   ├── handlers/
│   │   ├── ollama.go
│   │   └── transaction.go
│   ├── models/transaction.go
│   ├── repository/transaction.go
│   └── routes/routes.go
├── keywords.json
├── flake.nix
├── flake.lock
├── go.mod
├── go.sum
└── README.md
```

### Active Components

- `cmd/main.go`: process entry point, dependency startup, scheduling, HTTP server, and static frontend routes.
- `internal/db/db.go`: global SQLite connection and schema initialization.
- `internal/email/check.go`: Gmail connection, message discovery, MIME parsing, ingestion, and orchestration.
- `internal/email/filter.go`: lazy loading of keyword configuration and receipt classification.
- `internal/email/ollama.go`: prompt construction and Ollama transaction extraction.
- `internal/models/transaction.go`: shared transaction representation.
- `internal/repository/transaction.go`: all transaction SQL and duplicate detection.
- `internal/handlers/*.go`: transaction and Ollama HTTP handlers.
- `internal/routes/routes.go`: API route registration.
- `keywords.json`: receipt and junk keyword lists.

### Inactive or Placeholder Components

- `internal/email/parser.go` implements a regex-based parser for merchant, Indonesian rupiah amounts, and dates, but no active code calls `ParseEmail`. Ollama failures currently cause the email to be skipped rather than falling back to this parser.
- `config/config.go` contains only an empty `config` package declaration and is not imported.
- `credentials.json` and `token.json` are ignored for possible OAuth use, but the active implementation only supports Gmail username/app-password authentication.
- `frontend-folder/` is referenced by `cmd/main.go`, but no frontend files or directory are tracked in this repository.

## Process Lifecycle

`cmd/main.go` performs these steps in order:

1. Calls `godotenv.Load()` to load `.env` from the current working directory. Failure is logged and is not fatal.
2. Calls `db.Init()` synchronously. Database connection or schema creation failures terminate the process.
3. Starts an initial `email.Check()` in a background goroutine.
4. Creates a cron scheduler that runs `email.Check()` every 30 minutes.
5. Creates a Gin engine with the default logger and recovery middleware.
6. Registers API and static-file routes.
7. Listens on the fixed address `:7225`.

The initial email scan and HTTP server can run concurrently. Scheduled checks are not protected by a lock, so scans can overlap if a check lasts longer than 30 minutes. There is no explicit graceful shutdown, signal handling, database close, or error handling around `r.Run`.

## Configuration

Only Gmail credentials are configurable at runtime:

| Variable | Required for email scans | Purpose |
|---|---:|---|
| `SCALAR_EMAIL` | Yes | Gmail address used for IMAP login |
| `SCALAR_APP_PASSWORD` | Yes | Gmail app password used for IMAP login |

If either variable is missing, the server still starts, but each email check logs a message and returns immediately.

Most behavior is hard-coded:

| Setting | Current value |
|---|---|
| IMAP server | `imap.gmail.com:993` with TLS |
| Mailbox | `INBOX` |
| Email search window | Previous 14 days |
| Scan interval | 30 minutes |
| HTTP listen address | `:7225` |
| SQLite path | `scalar.db` |
| Keyword path | `keywords.json` |
| Ollama base URL | `http://localhost:11434` |
| Extraction model | `llama3:8b` |
| Confirmed prompt examples | 10 |
| Static frontend directory | `./frontend-folder` |

The database, keywords, `.env`, and frontend paths are relative to the process working directory. The application should therefore normally be started from the repository root. Starting elsewhere can create a different database and prevent keywords or frontend assets from being found.

## Email Ingestion

### Connection and Search

`email.Check`:

1. Reads the two Gmail environment variables.
2. Opens a TLS connection to Gmail IMAP.
3. Logs into the account and selects `INBOX` read-only.
4. Searches for all messages received since 14 days before the current time.
5. Fetches envelopes for the matching sequence numbers.
6. Sorts all fetched messages newest first by envelope date.
7. Processes candidates sequentially.

The variable named `uids` in `check.go` contains normal IMAP sequence numbers returned by `Search`; the following `SeqSet` and `SeqNum` usage is consistent with that behavior.

The application does not mark messages as read, move them, add labels, or maintain an IMAP cursor. Every scan searches the entire rolling 14-day window and relies on database duplicate checks to avoid reinsertion.

### Receipt Filtering

`internal/email/filter.go` loads `keywords.json` once per process through `sync.Once`. Changes to the file require a process restart.

Filtering rules are:

1. Lowercase the email subject and first sender address.
2. If any junk keyword occurs in either the subject or sender, reject the email.
3. Otherwise, if any expense keyword occurs in the subject, accept it.
4. Reject all other emails.

Matching is simple substring matching. Junk keywords always take precedence. Expense keywords are not checked against the sender. Keyword values are not normalized after loading, so custom values should be lowercase.

Failure to read or parse `keywords.json` calls `log.Fatalf`, terminating the process even if loading was triggered by the background checker.

### Message Identity

For an accepted envelope, the code hashes the IMAP `Message-Id` using SHA-256 and stores its lowercase hexadecimal representation as `email_hash`. `repository.ExistsByHash` skips hashes already in the database.

If a message has an empty message ID, the empty string is hashed. All messages with missing IDs therefore share the same hash and can incorrectly suppress one another.

### MIME Body Extraction

After the hash check, the service fetches the full body and parses it with `go-message/mail`:

- The first inline `text/plain` part is returned immediately.
- If no plain-text part exists, the last inline `text/html` part is used.
- Attachments and other part types are ignored.
- HTML tags are removed with a regular expression.
- A small set of entities (`&nbsp;`, `&amp;`, `&lt;`, `&gt;`, `&#39;`, and `&quot;`) is decoded.
- Whitespace is collapsed before the HTML fallback is returned.

There is no body-size limit or truncation before sending content to Ollama. The HTML conversion is intentionally basic and is not a full HTML parser.

### Ollama Extraction

`AskOllama` first loads up to ten confirmed transactions, ordered by most recently updated. These become few-shot examples containing merchant, category, and expense direction.

It then sends a non-streaming `POST` request to:

```text
http://localhost:11434/api/generate
```

The request selects the hard-coded `llama3:8b` model and asks Ollama to enforce this result schema:

```json
{
  "merchant": "string",
  "amount": 0,
  "category": "food & drink | transport | utilities | entertainment | shopping | health | transfer | other",
  "is_expense": true
}
```

The prompt includes the full extracted subject and body, category guidance, merchant normalization rules, amount instructions, and confirmed examples. Ollama's outer JSON response is decoded, then its `response` string is decoded again as the transaction result.

The client does not set an HTTP timeout, check the generation response status, or validate the decoded values after unmarshalling. Empty fields and zero values can therefore be stored if Ollama returns syntactically valid but incomplete data. The raw model response is written to application logs.

### Semantic Duplicate Detection

Before insertion, `repository.IsDuplicateTransaction` searches existing rows having exactly the same numeric amount and date. A matching row is considered a duplicate when either:

- One lowercased merchant name contains the other, or
- The merchant names share any ASCII alphanumeric word at least four characters long.

This is deliberately fuzzy but can produce false positives for common merchant words. It ignores non-ASCII letters during tokenization, requires exact floating-point amount equality, and has no currency dimension.

Skipped semantic duplicates are not recorded by email hash, so they will be fetched, parsed, and sent to Ollama again during later scans until they leave the 14-day window.

### Stored Transaction

The inserted transaction uses:

- `email_hash`: SHA-256 of the IMAP message ID
- `merchant`, `amount`, `category`, `is_expense`: Ollama output
- `date`: the email envelope date formatted as `YYYY-MM-DD`
- `description`: empty by default
- `confirmed`: database default `false`
- `updated_at`: database default current timestamp

The unused regex parser's extracted date is not part of the active flow.

## Database

`internal/db/db.go` opens `scalar.db` through a package-global `*sql.DB` and creates this table:

```sql
CREATE TABLE IF NOT EXISTS transactions (
    id          INTEGER PRIMARY KEY AUTOINCREMENT,
    email_hash  VARCHAR(64) UNIQUE NOT NULL,
    merchant    VARCHAR(255),
    amount      NUMERIC,
    category    VARCHAR(100),
    is_expense  BOOLEAN,
    date        TIMESTAMP,
    description TEXT,
    confirmed   BOOLEAN DEFAULT FALSE,
    updated_at  TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);
```

Every startup then attempts `ALTER TABLE transactions ADD COLUMN description TEXT` and ignores the error. This supports an older database that lacked the column, but on new/current databases it predictably fails because the column already exists.

There is no schema-version table or migration tool. There are no indexes beyond the primary key and unique `email_hash`. SQLite pool limits, busy timeout, and WAL mode are not explicitly configured.

### Model Shape

API and repository code represent a transaction as:

```json
{
  "id": 1,
  "email_hash": "sha256 hex string",
  "merchant": "Merchant",
  "amount": 123.45,
  "category": "shopping",
  "is_expense": true,
  "date": "2026-09-07",
  "description": "optional user description",
  "confirmed": false,
  "updated_at": "SQLite timestamp string"
}
```

Dates and timestamps are Go strings rather than `time.Time`. Amounts are `float64`; currency is neither extracted nor stored. SQL columns other than `id` and `email_hash` are nullable, but repository scans target non-nullable Go fields, so manually introduced nulls can cause rows to be skipped or return errors.

### Repository Operations

- `InsertTransaction` uses `INSERT OR IGNORE`. A unique-hash conflict returns no error and is indistinguishable from a successful insert to the caller.
- `ExistsByHash` checks exact hash presence.
- `IsDuplicateTransaction` performs the fuzzy merchant check described above.
- `GetAll` returns all rows ordered by date descending, with no pagination or deterministic tie-breaker.
- `GetByID` returns `nil, nil` when no row exists.
- `GetConfirmedExamples` returns the most recently updated confirmed rows for prompting.
- `UpdateByID` updates one row's category, expense flag, description, and timestamp without changing confirmation.
- `UpdateByMerchant` updates all rows with an exactly equal, case-sensitive merchant name and can set confirmation.

The duplicate check and insert are separate, non-transactional operations. The unique hash protects identical email IDs during overlapping scans, but semantic duplicates with different hashes can race and both be inserted.

## HTTP API

The server uses Gin's default middleware and listens on `:7225`. There is no authentication, authorization, CORS configuration, API version negotiation, rate limiting, or TLS termination in this repository.

### `GET /api/transactions`

Returns every transaction ordered by `date DESC`.

Success:

```text
200 OK
```

The response is a JSON array of transaction objects. On an empty database, the repository's nil slice may serialize as `null` rather than `[]`.

Database failure:

```json
{
  "error": "database error text"
}
```

### `PATCH /api/transactions/:id`

Accepted request fields:

```json
{
  "category": "food & drink",
  "is_expense": true,
  "description": "lunch",
  "confirmed": true
}
```

Actual behavior depends on `confirmed`:

- If `confirmed` is exactly `true`, all rows whose merchant exactly matches the selected row are updated and marked confirmed.
- If `confirmed` is omitted, `false`, or `null`, only the selected row is updated and its existing confirmation state is preserved.

Success response:

```json
{
  "message": "updated",
  "affected": 1
}
```

Important PATCH behavior: `category`, `is_expense`, and `description` are non-pointer Go fields. Although the README describes them as optional, omitted fields become their zero values and overwrite stored values with an empty string or `false`. This endpoint therefore does not implement true partial-update semantics.

There is no category validation and no API operation that explicitly unconfirms a transaction. Single-row updates do not create confirmed few-shot examples. Bulk-confirmed rows are used by future Ollama prompts.

Errors include invalid IDs (`400`), invalid JSON (`400`), missing transactions (`404`), and raw database errors (`500`).

### `GET /api/models`

Proxies Ollama's model list from:

```text
GET http://localhost:11434/api/tags
```

On success it returns Ollama's parsed JSON object unchanged. Connection, response-read, or JSON-decoding failures produce `500` responses; a non-200 Ollama status is passed through. This endpoint has no timeout or authentication and is not currently documented in `README.md`.

### Static Routes

- `/static/*` serves files from `./frontend-folder`.
- `/` serves `./frontend-folder/index.html`.

Because that directory is not tracked, a clean checkout does not contain the expected frontend.

## Correction and Learning Loop

The application's only learning mechanism is prompt-based few-shot feedback:

1. A client patches a transaction with `"confirmed": true`.
2. Every row with the exact same merchant is updated and marked confirmed.
3. Later email extractions query up to ten confirmed rows by recent `updated_at`.
4. Their merchant/category/expense mappings are inserted into the next Ollama prompt.

No model training or fine-tuning occurs. Confirmed examples are not deduplicated by merchant, and their amounts are queried from SQLite but not included in the generated example text.

## Development and Running

Expected local services and credentials:

- Go matching the module requirement
- Ollama listening on port `11434`
- The `llama3:8b` Ollama model installed
- Gmail account with an app password
- `SCALAR_EMAIL` and `SCALAR_APP_PASSWORD` set in `.env` or the environment

Run from the repository root:

```bash
go run cmd/main.go
```

Standard verification commands are:

```bash
go test ./...
go vet ./...
```

There are currently no `_test.go` files, so `go test ./...` only builds and initializes package test binaries.

### Nix Development Shell

`nix develop` provides:

- Go
- `gopls`
- Go tools
- `golangci-lint`
- Delve
- Air
- SQLite CLI
- Ollama

The flake pins `nixpkgs-unstable` and `flake-utils` through `flake.lock`. It provides tools only; it does not configure credentials, pull the model, start Ollama, initialize data, or run the application.

## Git-Ignored Local Data

`.gitignore` excludes:

- `.env`
- `scalar.db`
- `credentials.json`
- `token.json`

Do not commit Gmail credentials, app passwords, OAuth credentials, tokens, or the personal transaction database.

## README Differences

When README documentation and code disagree, the current code behaves as follows:

- The active Ollama model is `llama3:8b`, not `qwen3:1.7b`.
- `GET /api/models` exists but is absent from the README API table.
- PATCH only performs a merchant-wide confirmed update when the request contains `"confirmed": true`.
- PATCH fields are not safely optional; omitted editable fields overwrite values with zero values.
- No `.env.example`, `credentials.json` placeholder, or frontend directory is tracked.
- `internal/email/parser.go` is not an active fallback.

## Current Limitations and Risks

- No automated tests or external-service mocks.
- No CI, deployment, container, or production configuration.
- No API authentication; anyone able to reach the service can read and modify transactions.
- No network deadlines for IMAP or Ollama requests.
- No retry policy or exponential backoff.
- No graceful shutdown or application-level cancellation.
- Scheduled scans can overlap.
- Deduplication checks and writes are not atomic.
- Semantic duplicates are repeatedly reprocessed because their hashes are not persisted.
- `INSERT OR IGNORE` hides whether insertion occurred.
- LLM output is not semantically validated before storage.
- Email content is embedded directly in the prompt and can interfere with extraction instructions.
- Raw LLM responses and merchant/amount details are logged.
- No currency field; amounts use floating point.
- No pagination or filtering on transaction reads.
- PATCH has replacement-like zero-value behavior despite being named PATCH.
- No formal database migration framework.
- Relative paths make behavior dependent on the launch directory.
- Missing message IDs collide on the hash of an empty string.
- The HTML-to-text conversion and keyword classifier are intentionally simplistic.
- The clean checkout has no working frontend despite registered static routes.

## Guidance for Future Changes

- Preserve the package layering unless a change justifies restructuring: handlers call repositories, ingestion calls repositories/Ollama, and only the database package owns connection initialization.
- Treat `scalar.db` compatibility as persisted-data compatibility before changing schema or deduplication behavior.
- If changing PATCH semantics, account for existing clients and use pointer fields or an explicit update type to distinguish omitted values.
- If making model selection configurable, update both extraction and model-list behavior plus documentation.
- Use explicit contexts and timeouts for all IMAP and HTTP operations.
- Add tests around filtering, MIME extraction, duplicate matching, PATCH semantics, and Ollama response handling before broad refactors.
- Keep secrets and personal financial data out of source control and test fixtures.

## Verification Status

This context was produced by reviewing all tracked source, configuration, documentation, and dependency files on the `main` branch at commit `46f38d3` (`add available models route`). At creation time the ambient shell did not have `go` installed, so `go test ./...` and `go vet ./...` could not be executed outside the provided Nix development shell.
