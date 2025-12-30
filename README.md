# PBAKTH

A self-hosted financial tracking application built with Go backend and vanilla HTML/CSS/JS frontend.

## Features

- Expense and income tracking with customizable categories
- Recurring transactions
- Monthly visualization with pie charts and cashflow indicators
- Trial balance statement for accounting view
- CSV import/export
- Multi-language support
- Light and dark themes
- PWA support for mobile installation
- Multiple storage backends (JSON file or PostgreSQL)

## Quick Start

### Docker

```bash
docker run --rm -d \
  --name pbakth \
  -p 8080:8080 \
  -v pbakth-data:/app/data \
  tanq16/expenseowl:main
```

### Building from Source

```bash
git clone <your-repo-url>
cd expenseowl
go build ./cmd/expenseowl
./expenseowl
```

Access the application at `http://localhost:8080`

## Configuration

### Storage Backend

Default is JSON file storage. To use PostgreSQL:

```bash
STORAGE_TYPE=postgres
STORAGE_URL=localhost:5432/dbname
STORAGE_USER=username
STORAGE_PASS=password
STORAGE_SSL=disable
```

### Custom Port

```bash
./expenseowl -port 9000
```

## Architecture

- **Backend**: Go with Storage interface pattern for multiple backends
- **Frontend**: Vanilla HTML/CSS/JS (no build step required)
- **Data**: JSON files or PostgreSQL
- **Charts**: Chart.js for visualizations

## Development Branch

This project uses `modified` as the main development branch.

## Note

This application does not include authentication. Use behind a reverse proxy with authentication (Authelia, etc.) for production deployments.
