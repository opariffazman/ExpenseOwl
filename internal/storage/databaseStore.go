package storage

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"slices"
	"time"

	"github.com/google/uuid"
	"github.com/lib/pq"
)

// databaseStore implements the Storage interface for PostgreSQL.
type databaseStore struct {
	db       *sql.DB
	defaults map[string]string // allows reusing defaults without querying for config
}

// SQL queries as constants for reusability and clarity.
const (
	createExpensesTableSQL = `
	CREATE TABLE IF NOT EXISTS expenses (
		id VARCHAR(36) PRIMARY KEY,
		recurring_id VARCHAR(36),
		description VARCHAR(255) NOT NULL,
		"from" VARCHAR(255),
		"to" VARCHAR(255),
		method VARCHAR(50),
		note TEXT,
		category VARCHAR(255) NOT NULL,
		amount NUMERIC(10, 2) NOT NULL,
		currency VARCHAR(3) NOT NULL,
		date TIMESTAMPTZ NOT NULL
	);`

	createRecurringExpensesTableSQL = `
	CREATE TABLE IF NOT EXISTS recurring_expenses (
		id VARCHAR(36) PRIMARY KEY,
		description VARCHAR(255) NOT NULL,
		amount NUMERIC(10, 2) NOT NULL,
		currency VARCHAR(3) NOT NULL,
		"from" VARCHAR(255),
		"to" VARCHAR(255),
		method VARCHAR(50),
		note TEXT,
		category VARCHAR(255) NOT NULL,
		start_date TIMESTAMPTZ NOT NULL,
		interval VARCHAR(50) NOT NULL,
		occurrences INTEGER NOT NULL
	);`

	createConfigTableSQL = `
	CREATE TABLE IF NOT EXISTS config (
		id VARCHAR(255) PRIMARY KEY DEFAULT 'default',
		categories TEXT NOT NULL,
		currency VARCHAR(255) NOT NULL,
		start_date INTEGER NOT NULL,
		voucher_counter INTEGER DEFAULT 0,
		receipt_counter INTEGER DEFAULT 0,
		opening_balance DECIMAL(15,2) DEFAULT 0,
		use_manual_balances BOOLEAN DEFAULT false,
		manual_balances JSONB DEFAULT '{}'::jsonb
	);`
)

func InitializePostgresStore(baseConfig SystemConfig) (Storage, error) {
	dbURL := makeDBURL(baseConfig)
	db, err := sql.Open("postgres", dbURL)
	if err != nil {
		return nil, fmt.Errorf("failed to open PostgreSQL database: %v", err)
	}
	if err := db.Ping(); err != nil {
		return nil, fmt.Errorf("failed to ping PostgreSQL database: %v", err)
	}
	log.Println("Connected to PostgreSQL database")

	if err := createTables(db); err != nil {
		return nil, fmt.Errorf("failed to create database tables: %v", err)
	}
	return &databaseStore{db: db, defaults: map[string]string{}}, nil
}

func makeDBURL(baseConfig SystemConfig) string {
	return fmt.Sprintf("postgres://%s:%s@%s?sslmode=%s", baseConfig.StorageUser, baseConfig.StoragePass, baseConfig.StorageURL, baseConfig.StorageSSL)
}

func createTables(db *sql.DB) error {
	for _, query := range []string{createExpensesTableSQL, createRecurringExpensesTableSQL, createConfigTableSQL} {
		if _, err := db.Exec(query); err != nil {
			return err
		}
	}

	// Migration: Add new columns if they don't exist
	migrations := []string{
		`ALTER TABLE config ADD COLUMN IF NOT EXISTS use_manual_balances BOOLEAN DEFAULT false`,
		`ALTER TABLE config ADD COLUMN IF NOT EXISTS manual_balances JSONB DEFAULT '{}'::jsonb`,
	}
	for _, migration := range migrations {
		if _, err := db.Exec(migration); err != nil {
			return err
		}
	}

	return nil
}

func (s *databaseStore) Close() error {
	return s.db.Close()
}

func (s *databaseStore) saveConfig(config *Config) error {
	categoriesJSON, err := json.Marshal(config.Categories)
	if err != nil {
		return fmt.Errorf("failed to marshal categories: %v", err)
	}
	query := `
		INSERT INTO config (id, categories, currency, start_date)
		VALUES ('default', $1, $2, $3)
		ON CONFLICT (id) DO UPDATE SET
			categories = EXCLUDED.categories,
			currency = EXCLUDED.currency,
			start_date = EXCLUDED.start_date;
	`
	_, err = s.db.Exec(query, string(categoriesJSON), config.Currency, config.StartDate)
	s.defaults["currency"] = config.Currency
	s.defaults["start_date"] = fmt.Sprintf("%d", config.StartDate)
	return err
}

func (s *databaseStore) updateConfig(updater func(c *Config) error) error {
	config, err := s.GetConfig()
	if err != nil {
		return err
	}
	if err := updater(config); err != nil {
		return err
	}
	return s.saveConfig(config)
}

func (s *databaseStore) GetConfig() (*Config, error) {
	query := `SELECT categories, currency, start_date, COALESCE(voucher_counter, 0), COALESCE(receipt_counter, 0), COALESCE(opening_balance, 0), COALESCE(use_manual_balances, false), COALESCE(manual_balances, '{}'::jsonb) FROM config WHERE id = 'default'`
	var categoriesStr, currency, manualBalancesStr string
	var startDate, voucherCounter, receiptCounter int
	var openingBalance float64
	var useManualBalances bool
	err := s.db.QueryRow(query).Scan(&categoriesStr, &currency, &startDate, &voucherCounter, &receiptCounter, &openingBalance, &useManualBalances, &manualBalancesStr)

	if err != nil {
		if err == sql.ErrNoRows {
			config := &Config{}
			config.SetBaseConfig()
			if err := s.saveConfig(config); err != nil {
				return nil, fmt.Errorf("failed to save initial default config: %v", err)
			}
			return config, nil
		}
		return nil, fmt.Errorf("failed to get config from db: %v", err)
	}

	var config Config
	config.Currency = currency
	config.StartDate = startDate
	config.VoucherCounter = voucherCounter
	config.ReceiptCounter = receiptCounter
	config.OpeningBalance = openingBalance
	config.UseManualBalances = useManualBalances
	if err := json.Unmarshal([]byte(categoriesStr), &config.Categories); err != nil {
		return nil, fmt.Errorf("failed to parse categories from db: %v", err)
	}
	if err := json.Unmarshal([]byte(manualBalancesStr), &config.ManualBalances); err != nil {
		return nil, fmt.Errorf("failed to parse manual balances from db: %v", err)
	}

	recurring, err := s.GetRecurringExpenses()
	if err != nil {
		return nil, fmt.Errorf("failed to get recurring expenses for config: %v", err)
	}
	config.RecurringExpenses = recurring

	return &config, nil
}

func (s *databaseStore) GetCategories() ([]string, error) {
	config, err := s.GetConfig()
	if err != nil {
		return nil, err
	}
	return config.Categories, nil
}

func (s *databaseStore) UpdateCategories(categories []string) error {
	return s.updateConfig(func(c *Config) error {
		c.Categories = categories
		return nil
	})
}

func (s *databaseStore) GetCurrency() (string, error) {
	config, err := s.GetConfig()
	if err != nil {
		return "", err
	}
	return config.Currency, nil
}

func (s *databaseStore) UpdateCurrency(currency string) error {
	if !slices.Contains(SupportedCurrencies, currency) {
		return fmt.Errorf("invalid currency: %s", currency)
	}
	return s.updateConfig(func(c *Config) error {
		c.Currency = currency
		return nil
	})
}

func (s *databaseStore) GetStartDate() (int, error) {
	config, err := s.GetConfig()
	if err != nil {
		return 0, err
	}
	return config.StartDate, nil
}

func (s *databaseStore) UpdateStartDate(startDate int) error {
	if startDate < 1 || startDate > 31 {
		return fmt.Errorf("invalid start date: %d", startDate)
	}
	return s.updateConfig(func(c *Config) error {
		c.StartDate = startDate
		return nil
	})
}

func (s *databaseStore) GetLanguage() (string, error) {
	config, err := s.GetConfig()
	if err != nil {
		return "", err
	}
	if config.Language == "" {
		return "en", nil
	}
	return config.Language, nil
}

func (s *databaseStore) UpdateLanguage(language string) error {
	if !slices.Contains(SupportedLanguages, language) {
		return fmt.Errorf("invalid language: %s", language)
	}
	return s.updateConfig(func(c *Config) error {
		c.Language = language
		return nil
	})
}

func (s *databaseStore) GetOpeningBalance() (float64, error) {
	config, err := s.GetConfig()
	if err != nil {
		return 0, err
	}
	return config.OpeningBalance, nil
}

func (s *databaseStore) UpdateOpeningBalance(balance float64) error {
	query := `UPDATE config SET opening_balance = $1 WHERE id = 'default'`
	_, err := s.db.Exec(query, balance)
	return err
}

func (s *databaseStore) GetUseManualBalances() (bool, error) {
	config, err := s.GetConfig()
	if err != nil {
		return false, err
	}
	return config.UseManualBalances, nil
}

func (s *databaseStore) UpdateUseManualBalances(use bool) error {
	query := `UPDATE config SET use_manual_balances = $1 WHERE id = 'default'`
	_, err := s.db.Exec(query, use)
	return err
}

func (s *databaseStore) GetManualBalances() (map[string]float64, error) {
	config, err := s.GetConfig()
	if err != nil {
		return nil, err
	}
	if config.ManualBalances == nil {
		return make(map[string]float64), nil
	}
	return config.ManualBalances, nil
}

func (s *databaseStore) UpdateManualBalances(balances map[string]float64) error {
	balancesJSON, err := json.Marshal(balances)
	if err != nil {
		return fmt.Errorf("failed to marshal manual balances: %v", err)
	}
	query := `UPDATE config SET manual_balances = $1 WHERE id = 'default'`
	_, err = s.db.Exec(query, balancesJSON)
	return err
}

func scanExpense(scanner interface{ Scan(...any) error }) (Expense, error) {
	var expense Expense
	var recurringID, fromStr, toStr, methodStr, noteStr sql.NullString
	err := scanner.Scan(&expense.ID, &recurringID, &expense.Description, &fromStr, &toStr, &methodStr, &noteStr, &expense.Category, &expense.Amount, &expense.Date)
	if err != nil {
		return Expense{}, err
	}
	if recurringID.Valid {
		expense.RecurringID = recurringID.String
	}
	if fromStr.Valid {
		expense.From = fromStr.String
	}
	if toStr.Valid {
		expense.To = toStr.String
	}
	if methodStr.Valid {
		expense.Method = methodStr.String
	}
	if noteStr.Valid {
		expense.Note = noteStr.String
	}
	return expense, nil
}

func (s *databaseStore) GetAllExpenses() ([]Expense, error) {
	query := `SELECT id, recurring_id, description, "from", "to", method, note, category, amount, date FROM expenses ORDER BY date DESC`
	rows, err := s.db.Query(query)
	if err != nil {
		return nil, fmt.Errorf("failed to query expenses: %v", err)
	}
	defer rows.Close()

	var expenses []Expense
	for rows.Next() {
		expense, err := scanExpense(rows)
		if err != nil {
			return nil, fmt.Errorf("failed to scan expense: %v", err)
		}
		expenses = append(expenses, expense)
	}
	return expenses, nil
}

func (s *databaseStore) GetExpense(id string) (Expense, error) {
	query := `SELECT id, recurring_id, description, "from", "to", method, note, category, amount, date FROM expenses WHERE id = $1`
	expense, err := scanExpense(s.db.QueryRow(query, id))
	if err != nil {
		if err == sql.ErrNoRows {
			return Expense{}, fmt.Errorf("expense with ID %s not found", id)
		}
		return Expense{}, fmt.Errorf("failed to get expense: %v", err)
	}
	return expense, nil
}

func (s *databaseStore) AddExpense(expense Expense) error {
	if expense.ID == "" {
		// Generate ID based on transaction type
		isGain := expense.Amount > 0
		var counter int
		if isGain {
			s.db.QueryRow(`UPDATE config SET receipt_counter = receipt_counter + 1 RETURNING receipt_counter`).Scan(&counter)
		} else {
			s.db.QueryRow(`UPDATE config SET voucher_counter = voucher_counter + 1 RETURNING voucher_counter`).Scan(&counter)
		}
		expense.ID = GenerateTransactionID(isGain, counter)
	}
	if expense.Currency == "" {
		expense.Currency = s.defaults["currency"]
	}
	if expense.Date.IsZero() {
		expense.Date = time.Now()
	}
	query := `
		INSERT INTO expenses (id, recurring_id, description, "from", "to", method, note, category, amount, currency, date)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11)
	`
	_, err := s.db.Exec(query, expense.ID, expense.RecurringID, expense.Description, expense.From, expense.To, expense.Method, expense.Note, expense.Category, expense.Amount, expense.Currency, expense.Date)
	return err
}

func (s *databaseStore) UpdateExpense(id string, expense Expense) error {
	// TODO: revisit to maybe remove this later, might not be a good default for update
	if expense.Currency == "" {
		expense.Currency = s.defaults["currency"]
	}
	query := `
		UPDATE expenses
		SET description = $1, "from" = $2, "to" = $3, method = $4, note = $5, category = $6, amount = $7, currency = $8, date = $9, recurring_id = $10
		WHERE id = $11
	`
	result, err := s.db.Exec(query, expense.Description, expense.From, expense.To, expense.Method, expense.Note, expense.Category, expense.Amount, expense.Currency, expense.Date, expense.RecurringID, id)
	if err != nil {
		return fmt.Errorf("failed to update expense: %v", err)
	}
	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get rows affected: %v", err)
	}
	if rowsAffected == 0 {
		return fmt.Errorf("expense with ID %s not found", id)
	}
	return nil
}

func (s *databaseStore) RemoveExpense(id string) error {
	query := `DELETE FROM expenses WHERE id = $1`
	result, err := s.db.Exec(query, id)
	if err != nil {
		return fmt.Errorf("failed to delete expense: %v", err)
	}
	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get rows affected: %v", err)
	}
	if rowsAffected == 0 {
		return fmt.Errorf("expense with ID %s not found", id)
	}
	return nil
}

func (s *databaseStore) AddMultipleExpenses(expenses []Expense) error {
	if len(expenses) == 0 {
		return nil
	}
	// use the same addexpense method
	for _, exp := range expenses {
		if err := s.AddExpense(exp); err != nil {
			return err
		}
	}
	return nil
}

func (s *databaseStore) RemoveMultipleExpenses(ids []string) error {
	if len(ids) == 0 {
		return nil
	}
	query := `DELETE FROM expenses WHERE id = ANY($1)`
	_, err := s.db.Exec(query, pq.Array(ids))
	if err != nil {
		return fmt.Errorf("failed to delete multiple expenses: %v", err)
	}
	return nil
}

func scanRecurringExpense(scanner interface{ Scan(...any) error }) (RecurringExpense, error) {
	var re RecurringExpense
	var fromStr, toStr, methodStr, noteStr sql.NullString
	err := scanner.Scan(&re.ID, &re.Description, &re.Amount, &re.Currency, &fromStr, &toStr, &methodStr, &noteStr, &re.Category, &re.StartDate, &re.Interval, &re.Occurrences)
	if err != nil {
		return RecurringExpense{}, err
	}
	if fromStr.Valid {
		re.From = fromStr.String
	}
	if toStr.Valid {
		re.To = toStr.String
	}
	if methodStr.Valid {
		re.Method = methodStr.String
	}
	if noteStr.Valid {
		re.Note = noteStr.String
	}
	return re, nil
}

func (s *databaseStore) GetRecurringExpenses() ([]RecurringExpense, error) {
	query := `SELECT id, description, amount, currency, "from", "to", method, note, category, start_date, interval, occurrences FROM recurring_expenses`
	rows, err := s.db.Query(query)
	if err != nil {
		return nil, fmt.Errorf("failed to query recurring expenses: %v", err)
	}
	defer rows.Close()
	var recurringExpenses []RecurringExpense
	for rows.Next() {
		re, err := scanRecurringExpense(rows)
		if err != nil {
			return nil, fmt.Errorf("failed to scan recurring expense: %v", err)
		}
		recurringExpenses = append(recurringExpenses, re)
	}
	return recurringExpenses, nil
}

func (s *databaseStore) GetRecurringExpense(id string) (RecurringExpense, error) {
	query := `SELECT id, description, amount, currency, "from", "to", method, note, category, start_date, interval, occurrences FROM recurring_expenses WHERE id = $1`
	re, err := scanRecurringExpense(s.db.QueryRow(query, id))
	if err != nil {
		if err == sql.ErrNoRows {
			return RecurringExpense{}, fmt.Errorf("recurring expense with ID %s not found", id)
		}
		return RecurringExpense{}, fmt.Errorf("failed to get recurring expense: %v", err)
	}
	return re, nil
}

func (s *databaseStore) AddRecurringExpense(recurringExpense RecurringExpense) error {
	tx, err := s.db.Begin()
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %v", err)
	}
	defer tx.Rollback() // Rollback on error

	if recurringExpense.ID == "" {
		recurringExpense.ID = uuid.New().String()
	}
	if recurringExpense.Currency == "" {
		recurringExpense.Currency = s.defaults["currency"]
	}
	ruleQuery := `
		INSERT INTO recurring_expenses (id, description, amount, currency, "from", "to", method, note, category, start_date, interval, occurrences)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12)
	`
	_, err = tx.Exec(ruleQuery, recurringExpense.ID, recurringExpense.Description, recurringExpense.Amount, recurringExpense.Currency, recurringExpense.From, recurringExpense.To, recurringExpense.Method, recurringExpense.Note, recurringExpense.Category, recurringExpense.StartDate, recurringExpense.Interval, recurringExpense.Occurrences)
	if err != nil {
		return fmt.Errorf("failed to insert recurring expense rule: %v", err)
	}

	expensesToAdd := generateExpensesFromRecurring(recurringExpense, false)
	if len(expensesToAdd) > 0 {
		stmt, err := tx.Prepare(pq.CopyIn("expenses", "id", "recurring_id", "description", "from", "to", "method", "note", "category", "amount", "currency", "date"))
		if err != nil {
			return fmt.Errorf("failed to prepare copy in: %v", err)
		}
		defer stmt.Close()
		for _, exp := range expensesToAdd {
			_, err = stmt.Exec(exp.ID, exp.RecurringID, exp.Description, exp.From, exp.To, exp.Method, exp.Note, exp.Category, exp.Amount, exp.Currency, exp.Date)
			if err != nil {
				return fmt.Errorf("failed to execute copy in: %v", err)
			}
		}
		if _, err = stmt.Exec(); err != nil {
			return fmt.Errorf("failed to finalize copy in: %v", err)
		}
	}
	return tx.Commit()
}

func (s *databaseStore) UpdateRecurringExpense(id string, recurringExpense RecurringExpense, updateAll bool) error {
	tx, err := s.db.Begin()
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %v", err)
	}
	defer tx.Rollback()
	recurringExpense.ID = id // Ensure ID is preserved
	if recurringExpense.Currency == "" {
		recurringExpense.Currency = s.defaults["currency"]
	}
	ruleQuery := `
		UPDATE recurring_expenses
		SET description = $1, amount = $2, "from" = $3, "to" = $4, method = $5, note = $6, category = $7, start_date = $8, interval = $9, occurrences = $10, currency = $11
		WHERE id = $12
	`
	res, err := tx.Exec(ruleQuery, recurringExpense.Description, recurringExpense.Amount, recurringExpense.From, recurringExpense.To, recurringExpense.Method, recurringExpense.Note, recurringExpense.Category, recurringExpense.StartDate, recurringExpense.Interval, recurringExpense.Occurrences, recurringExpense.Currency, id)
	if err != nil {
		return fmt.Errorf("failed to update recurring expense rule: %v", err)
	}
	rowsAffected, _ := res.RowsAffected()
	if rowsAffected == 0 {
		return fmt.Errorf("recurring expense with ID %s not found to update", id)
	}

	var deleteQuery string
	if updateAll {
		deleteQuery = `DELETE FROM expenses WHERE recurring_id = $1`
		_, err = tx.Exec(deleteQuery, id)
	} else {
		deleteQuery = `DELETE FROM expenses WHERE recurring_id = $1 AND date > $2`
		_, err = tx.Exec(deleteQuery, id, time.Now())
	}
	if err != nil {
		return fmt.Errorf("failed to delete old expense instances for update: %v", err)
	}

	expensesToAdd := generateExpensesFromRecurring(recurringExpense, !updateAll)
	if len(expensesToAdd) > 0 {
		stmt, err := tx.Prepare(pq.CopyIn("expenses", "id", "recurring_id", "description", "from", "to", "method", "note", "category", "amount", "currency", "date"))
		if err != nil {
			return fmt.Errorf("failed to prepare copy in for update: %v", err)
		}
		defer stmt.Close()
		for _, exp := range expensesToAdd {
			_, err = stmt.Exec(exp.ID, exp.RecurringID, exp.Description, exp.From, exp.To, exp.Method, exp.Note, exp.Category, exp.Amount, exp.Currency, exp.Date)
			if err != nil {
				return fmt.Errorf("failed to execute copy in for update: %v", err)
			}
		}
		if _, err = stmt.Exec(); err != nil {
			return fmt.Errorf("failed to finalize copy in for update: %v", err)
		}
	}
	return tx.Commit()
}

func (s *databaseStore) RemoveRecurringExpense(id string, removeAll bool) error {
	tx, err := s.db.Begin()
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %v", err)
	}
	defer tx.Rollback()
	res, err := tx.Exec(`DELETE FROM recurring_expenses WHERE id = $1`, id)
	if err != nil {
		return fmt.Errorf("failed to delete recurring expense rule: %v", err)
	}
	rowsAffected, _ := res.RowsAffected()
	if rowsAffected == 0 {
		return fmt.Errorf("recurring expense with ID %s not found", id)
	}

	var deleteQuery string
	if removeAll {
		deleteQuery = `DELETE FROM expenses WHERE recurring_id = $1`
		_, err = tx.Exec(deleteQuery, id)
	} else {
		deleteQuery = `DELETE FROM expenses WHERE recurring_id = $1 AND date > $2`
		_, err = tx.Exec(deleteQuery, id, time.Now())
	}
	if err != nil {
		return fmt.Errorf("failed to delete expense instances: %v", err)
	}
	return tx.Commit()
}

func generateExpensesFromRecurring(recExp RecurringExpense, fromToday bool) []Expense {
	var expenses []Expense
	currentDate := recExp.StartDate
	today := time.Now()
	occurrencesToGenerate := recExp.Occurrences
	if fromToday {
		for currentDate.Before(today) && (recExp.Occurrences == 0 || occurrencesToGenerate > 0) {
			switch recExp.Interval {
			case "daily":
				currentDate = currentDate.AddDate(0, 0, 1)
			case "weekly":
				currentDate = currentDate.AddDate(0, 0, 7)
			case "monthly":
				currentDate = currentDate.AddDate(0, 1, 0)
			case "yearly":
				currentDate = currentDate.AddDate(1, 0, 0)
			default:
				return expenses // Stop if interval is invalid
			}
			if recExp.Occurrences > 0 {
				occurrencesToGenerate--
			}
		}
	}
	limit := occurrencesToGenerate
	// if recExp.Occurrences == 0 {
	// 	limit = 2000 // Heuristic for "indefinite"
	// }

	for range limit {
		expense := Expense{
			ID:          "", // Will be generated by AddMultipleExpenses
			RecurringID: recExp.ID,
			Description: recExp.Description,
			From:        recExp.From,
			To:          recExp.To,
			Method:      recExp.Method,
			Note:        recExp.Note,
			Category:    recExp.Category,
			Amount:      recExp.Amount,
			Currency:    recExp.Currency,
			Date:        currentDate,
		}
		expenses = append(expenses, expense)
		switch recExp.Interval {
		case "daily":
			currentDate = currentDate.AddDate(0, 0, 1)
		case "weekly":
			currentDate = currentDate.AddDate(0, 0, 7)
		case "monthly":
			currentDate = currentDate.AddDate(0, 1, 0)
		case "yearly":
			currentDate = currentDate.AddDate(1, 0, 0)
		default:
			return expenses
		}
	}
	return expenses
}
