# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

ExpenseOwl is a self-hosted expense tracking application built with Go backend and vanilla HTML/CSS/JS frontend. It emphasizes simplicity and speed, designed for single-user homelab deployments without complex features like budgeting or multi-account support.

## Repository Context

**Important**: This is a fork with customizations:
- **Main development branch**: `modified` (not `main`)
- **Project name**: References to "ExpenseOwl" have been changed to "PBAKTH" in this fork
- Always work in the `modified` branch unless explicitly instructed otherwise

## Development Commands

### Building and Running

```bash
# Build the binary
go build ./cmd/expenseowl

# Run the server (default port 8080)
./expenseowl

# Run with custom port
./expenseowl -port 9000

# Build Docker image
docker build -t expenseowl .

# Run via Docker
docker run --rm -d -p 8080:8080 -v expenseowl:/app/data expenseowl
```

### Testing

This project does not currently have automated tests.

### Development Workflow

**IMPORTANT**: After making code changes (especially to frontend files), always rebuild and run the server so the user can test changes in the browser immediately:

```bash
./run-server.sh
```

This is critical because:
- Frontend assets are embedded at compile time via `//go:embed`
- Changes to HTML/CSS/JS files won't be visible until the binary is rebuilt
- The user expects to test changes immediately in their browser

## Architecture

### Storage Interface Pattern

ExpenseOwl uses a **Storage interface** (`internal/storage/storage.go`) to abstract data persistence. This allows multiple backend implementations:

- **JSON Store** (`jsonStore.go`): File-based storage using `data/expenses.json` and `data/config.json`
- **PostgreSQL Store** (`databaseStore.go`): Database storage using PostgreSQL

The storage backend is selected via the `STORAGE_TYPE` environment variable (`json` or `postgres`). Adding new storage backends requires implementing the `Storage` interface.

### Key Storage Types

- `Expense`: Individual expense/income transaction with ID, description, category, amount, date, optional tags (from/to), and currency
- `RecurringExpense`: Template for recurring transactions that generates future expense instances
- `Config`: Application configuration including categories, currency, start date, and language

### Package Structure

```
cmd/expenseowl/main.go        - Entry point, HTTP route registration
internal/
  api/
    handlers.go               - HTTP handlers for all API endpoints
    import-export.go          - CSV import/export logic
  storage/
    storage.go                - Storage interface and core types
    jsonStore.go              - JSON file implementation
    databaseStore.go          - PostgreSQL implementation
  web/
    embed.go                  - Embedded static files (templates, CSS, JS)
    templates/                - Frontend HTML, CSS, JS, PWA assets
```

### HTTP API Patterns

All endpoints follow REST-like conventions:
- Config endpoints: `/config`, `/categories`, `/currency`, `/startdate`, `/language`
- Expense CRUD: `/expense` (PUT to add), `/expenses` (GET all), `/expense/edit` (PUT), `/expense/delete` (DELETE)
- Recurring: `/recurring-expense`, `/recurring-expenses`, `/recurring-expense/edit`, `/recurring-expense/delete`
- Import/Export: `/export/csv`, `/import/csv`, `/import/csvold`

All API handlers use `writeJSON()` helper for consistent JSON responses and error handling.

### Frontend Architecture

- **Embedded at compile time**: All frontend assets are embedded via Go's `//go:embed` directive
- **No build step**: Vanilla JS with no transpilation or bundling
- **Chart.js**: Used for pie chart visualization on the dashboard
- **PWA Support**: Includes service worker (`sw.js`) and manifest for mobile installation
- **Internationalization**: Supports multiple languages via `/locales/` JSON files

### Recurring Expenses Logic

Recurring expenses are stored as templates with `StartDate`, `Interval` (daily/weekly/monthly/yearly), and `Occurrences`. The `generateExpensesFromRecurring()` function creates individual `Expense` records:
- All future expense instances are created immediately in the database/JSON file
- Each generated expense has a `RecurringID` field linking back to its template
- When editing/deleting recurring expenses, you can choose to update "all instances" or "future only"

### Data Validation

All user input is sanitized via `SanitizeString()`:
- Uses regex to allow only unicode letters, numbers, spaces, and basic punctuation
- Removes repeating spaces and trims whitespace
- Applied to description, from/to fields, and category names

Validation is enforced at the storage layer via `Validate()` methods on `Expense` and `RecurringExpense`.

## Configuration via Environment Variables

PostgreSQL backend is configured via:
- `STORAGE_TYPE`: `json` (default) or `postgres`
- `STORAGE_URL`: Database URL in format `host:port/dbname` (defaults to `data` for JSON)
- `STORAGE_SSL`: `disable` (default), `require`, `verify-full`, `verify-ca`
- `STORAGE_USER`: PostgreSQL username
- `STORAGE_PASS`: PostgreSQL password

## Important Conventions

### Date Handling
- Dates are stored as `time.Time` in RFC3339 format (UTC)
- Frontend hides time component; users select date and current local time is added
- CSV imports accept RFC3339 or human-readable formats like `2024/12/28` (must be YYYY-MM-DD order)

### Expense Sign Convention
- Negative amounts = expenses
- Positive amounts = income/reimbursement (via "Report as gain" checkbox)

### Start Date Feature
- Configurable day of month (1-31) when the "expense month" begins
- Example: Start date of 5 means monthly view shows 5th of current month to 4th of next month

### CSV Import/Export
- Export includes IDs to allow re-importing without duplicates
- Import requires: `name`, `description`, `category`, `amount`, `date` columns (case-insensitive)
- 10ms delay per record during import to reduce disk/DB overhead

## Key Implementation Notes

### Thread Safety
- `jsonStore` uses `sync.RWMutex` for concurrent access
- PostgreSQL store relies on database ACID guarantees

### Transaction Handling
- Recurring expense operations use database transactions to ensure atomicity
- JSON store operations are atomic at the file level (read-modify-write with mutex)

### ID Generation
- All expenses and recurring expenses use UUIDs (`github.com/google/uuid`)

### Default Values
- Both stores cache default currency and start date in memory (`defaults` map)
- Applied automatically when creating expenses if not specified

## Project Philosophy

When contributing or modifying:
- Maintain extreme simplicity - no unnecessary features
- Frontend must remain vanilla HTML/CSS/JS (no build step)
- Support self-hosted first (container and binary)
- No authentication built-in (use reverse proxy with Authelia/etc)
- Monthly pie chart is the primary interface - support this use case above all
