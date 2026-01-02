package storage

import (
	"fmt"
	"os"
	"regexp"
	"strings"
	"time"
)

// Storage interface for all storage types
type Storage interface {
	Close() error
	GetConfig() (*Config, error)

	// Basic Config Updates
	GetCategories() ([]string, error)
	UpdateCategories(categories []string) error
	// GetTags() ([]string, error)
	// UpdateTags(tags []string) error
	GetCurrency() (string, error)
	UpdateCurrency(currency string) error
	GetStartDate() (int, error)
	UpdateStartDate(startDate int) error
	GetLanguage() (string, error)
	UpdateLanguage(language string) error
	GetOpeningBalance() (float64, error)
	UpdateOpeningBalance(balance float64) error
	GetUseManualBalances() (bool, error)
	UpdateUseManualBalances(use bool) error
	GetManualBalances() (map[string]float64, error)
	UpdateManualBalances(balances map[string]float64) error

	// Expenses
	GetAllExpenses() ([]Expense, error)
	GetExpense(id string) (Expense, error)
	AddExpense(expense Expense) error
	RemoveExpense(id string) error
	AddMultipleExpenses(expenses []Expense) error
	RemoveMultipleExpenses(ids []string) error
	UpdateExpense(id string, expense Expense) error

	// Potential Future Feature: Multi-currency
	// GetConversions() (map[string]float64, error)
	// UpdateConversions(conversions map[string]float64) error
}

// config for expense data
type Config struct {
	Categories        []string           `json:"categories"`
	Currency          string             `json:"currency"`
	StartDate         int                `json:"startDate"`
	Language          string             `json:"language"`
	VoucherCounter    int                `json:"voucherCounter"`    // Counter for BAU (Baucar/voucher) IDs
	ReceiptCounter    int                `json:"receiptCounter"`    // Counter for RES (Resit/receipt) IDs
	OpeningBalance    float64            `json:"openingBalance"`    // Opening balance for statement generation
	UseManualBalances bool               `json:"useManualBalances"` // Toggle for manual category balances feature
	ManualBalances    map[string]float64 `json:"manualBalances"`    // Manual final balances per category
	// Tags              []string           `json:"tags"`
}

type BackendType string

const (
	BackendTypeJSON     BackendType = "json"
	BackendTypePostgres BackendType = "postgres"
)

// config for the storage backend
type SystemConfig struct {
	StorageURL  string
	StorageType BackendType
	StorageUser string
	StoragePass string
	StorageSSL  string
}

// expense struct
type Expense struct {
	ID          string    `json:"id"`
	Description string    `json:"description"`
	From        string    `json:"from"`
	To          string    `json:"to"`
	Method      string    `json:"method"`
	Note        string    `json:"note"` // Required for cheque and transfer methods
	Category    string    `json:"category"`
	Amount      float64   `json:"amount"`
	Currency    string    `json:"currency"`
	Date        time.Time `json:"date"`
}

// GenerateTransactionID generates a transaction ID based on whether it's an expense (BAU) or gain (RES)
func GenerateTransactionID(isGain bool, counter int) string {
	prefix := "BAU" // Baucar (voucher) for expenses
	if isGain {
		prefix = "RES" // Resit (receipt) for gains
	}
	return fmt.Sprintf("%s-%04d", prefix, counter)
}

func (c *Config) SetBaseConfig() {
	c.Categories = defaultCategories
	c.Currency = "usd"
	c.StartDate = 1
	c.Language = "en"
	// c.Tags = []string{}
}

func (c *SystemConfig) SetStorageConfig() {
	c.StorageType = backendTypeFromEnv(os.Getenv("STORAGE_TYPE"))
	c.StorageURL = backendURLFromEnv(os.Getenv("STORAGE_URL"))
	c.StorageSSL = backendSSLFromEnv(os.Getenv("STORAGE_SSL"))
	c.StorageUser = os.Getenv("STORAGE_USER")
	c.StoragePass = os.Getenv("STORAGE_PASS")
}

func backendTypeFromEnv(env string) BackendType {
	switch env {
	case "json":
		return BackendTypeJSON
	case "postgres":
		return BackendTypePostgres
	default:
		return BackendTypeJSON
	}
}

func backendURLFromEnv(env string) string {
	if env == "" {
		return "data"
	}
	return env
}

func backendSSLFromEnv(env string) string {
	switch env {
	case "disable", "require", "verify-full", "verify-ca":
		return env
	default:
		return "disable"
	}
}

// initializes the storage backend
func InitializeStorage() (Storage, error) {
	baseConfig := SystemConfig{}
	baseConfig.SetStorageConfig()
	switch baseConfig.StorageType {
	case BackendTypeJSON:
		return InitializeJsonStore(baseConfig)
	case BackendTypePostgres:
		return InitializePostgresStore(baseConfig)
	}
	return nil, fmt.Errorf("invalid data store: %s", baseConfig.StorageType)
}

var REInvalidChars *regexp.Regexp = regexp.MustCompile(`[^\p{L}\p{N}\s.,\-'_!"&]`)
var RERepeatingSpaces *regexp.Regexp = regexp.MustCompile(`\s+`)

// allows readable chars like unicode, otherwise replaces with whitespace
func SanitizeString(s string) string {
	sanitized := REInvalidChars.ReplaceAllString(s, " ")
	sanitized = RERepeatingSpaces.ReplaceAllString(sanitized, " ")
	return strings.TrimSpace(sanitized)
}

func ValidateCategory(category string) (string, error) {
	sanitized := SanitizeString(category)
	if sanitized == "" {
		return "", fmt.Errorf("category name cannot be empty or contain only invalid characters")
	}
	return sanitized, nil
}

func (e *Expense) Validate() error {
	e.Description = SanitizeString(e.Description)
	if e.Description == "" {
		return fmt.Errorf("expense 'description' cannot be empty")
	}
	e.From = SanitizeString(e.From)
	e.To = SanitizeString(e.To)
	e.Method = SanitizeString(e.Method)
	if e.Category == "" {
		return fmt.Errorf("expense 'category' cannot be empty")
	}
	if e.Amount == 0 {
		return fmt.Errorf("expense 'amount' cannot be 0")
	}
	// if e.Currency == "" {
	// 	return fmt.Errorf("expense 'currency' cannot be empty")
	// }
	if e.Date.IsZero() {
		return fmt.Errorf("expense 'date' cannot be empty")
	}
	return nil
}

// variables
var defaultCategories = []string{
	"Food",
	"Groceries",
	"Travel",
	"Rent",
	"Utilities",
	"Entertainment",
	"Healthcare",
	"Shopping",
	"Miscellaneous",
	"Income",
}

var SupportedLanguages = []string{
	"en", // English
	"ms", // Bahasa Malaysia
}

var SupportedCurrencies = []string{
	"usd", // US Dollar
	"eur", // Euro
	"gbp", // British Pound
	"jpy", // Japanese Yen
	"cny", // Chinese Yuan
	"krw", // Korean Won
	"inr", // Indian Rupee
	"rub", // Russian Ruble
	"brl", // Brazilian Real
	"zar", // South African Rand
	"aed", // UAE Dirham
	"aud", // Australian Dollar
	"cad", // Canadian Dollar
	"chf", // Swiss Franc
	"hkd", // Hong Kong Dollar
	"bdt", // Bangladeshi Taka
	"sgd", // Singapore Dollar
	"thb", // Thai Baht
	"try", // Turkish Lira
	"mxn", // Mexican Peso
	"php", // Philippine Peso
	"pln", // Polish Złoty
	"sek", // Swedish Krona
	"nzd", // New Zealand Dollar
	"dkk", // Danish Krone
	"idr", // Indonesian Rupiah
	"ils", // Israeli New Shekel
	"vnd", // Vietnamese Dong
	"myr", // Malaysian Ringgit
	"mad", // Moroccan Dirham
}
