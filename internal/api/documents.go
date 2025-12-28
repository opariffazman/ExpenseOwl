package api

import (
	"fmt"
	"log"
	"math"
	"net/http"
	"strings"
	"time"

	"github.com/johnfercher/maroto/v2"
	"github.com/johnfercher/maroto/v2/pkg/components/col"
	"github.com/johnfercher/maroto/v2/pkg/components/image"
	"github.com/johnfercher/maroto/v2/pkg/components/text"
	"github.com/johnfercher/maroto/v2/pkg/config"
	"github.com/johnfercher/maroto/v2/pkg/consts/align"
	"github.com/johnfercher/maroto/v2/pkg/consts/extension"
	"github.com/johnfercher/maroto/v2/pkg/consts/fontstyle"
	"github.com/johnfercher/maroto/v2/pkg/consts/pagesize"
	"github.com/johnfercher/maroto/v2/pkg/consts/orientation"
	"github.com/johnfercher/maroto/v2/pkg/props"
	"github.com/tanq16/expenseowl/internal/storage"
	"github.com/tanq16/expenseowl/internal/web"
)

// currencyBehavior defines how a currency should be formatted
type currencyBehavior struct {
	symbol      string
	useComma    bool
	useDecimals bool
	useSpace    bool
	right       bool
}

// currencyBehaviors maps currency codes to their formatting rules
var currencyBehaviors = map[string]currencyBehavior{
	"usd": {symbol: "$", useComma: false, useDecimals: true, useSpace: false, right: false},
	"eur": {symbol: "€", useComma: true, useDecimals: true, useSpace: false, right: false},
	"gbp": {symbol: "£", useComma: false, useDecimals: true, useSpace: false, right: false},
	"jpy": {symbol: "¥", useComma: false, useDecimals: false, useSpace: false, right: false},
	"cny": {symbol: "¥", useComma: false, useDecimals: true, useSpace: false, right: false},
	"krw": {symbol: "₩", useComma: false, useDecimals: false, useSpace: false, right: false},
	"inr": {symbol: "₹", useComma: false, useDecimals: true, useSpace: false, right: false},
	"rub": {symbol: "₽", useComma: true, useDecimals: true, useSpace: false, right: false},
	"brl": {symbol: "R$", useComma: true, useDecimals: true, useSpace: false, right: false},
	"zar": {symbol: "R", useComma: false, useDecimals: true, useSpace: true, right: true},
	"aed": {symbol: "AED", useComma: false, useDecimals: true, useSpace: true, right: true},
	"aud": {symbol: "A$", useComma: false, useDecimals: true, useSpace: false, right: false},
	"cad": {symbol: "C$", useComma: false, useDecimals: true, useSpace: false, right: false},
	"chf": {symbol: "Fr", useComma: false, useDecimals: true, useSpace: true, right: true},
	"hkd": {symbol: "HK$", useComma: false, useDecimals: true, useSpace: false, right: false},
	"bdt": {symbol: "৳", useComma: false, useDecimals: true, useSpace: false, right: false},
	"sgd": {symbol: "S$", useComma: false, useDecimals: true, useSpace: false, right: false},
	"thb": {symbol: "฿", useComma: false, useDecimals: true, useSpace: false, right: false},
	"try": {symbol: "₺", useComma: true, useDecimals: true, useSpace: false, right: false},
	"mxn": {symbol: "Mex$", useComma: false, useDecimals: true, useSpace: false, right: false},
	"php": {symbol: "₱", useComma: false, useDecimals: true, useSpace: false, right: false},
	"pln": {symbol: "zł", useComma: true, useDecimals: true, useSpace: true, right: true},
	"sek": {symbol: "kr", useComma: false, useDecimals: true, useSpace: true, right: true},
	"nzd": {symbol: "NZ$", useComma: false, useDecimals: true, useSpace: false, right: false},
	"dkk": {symbol: "kr.", useComma: true, useDecimals: true, useSpace: true, right: true},
	"idr": {symbol: "Rp", useComma: false, useDecimals: true, useSpace: true, right: true},
	"ils": {symbol: "₪", useComma: false, useDecimals: true, useSpace: false, right: false},
	"vnd": {symbol: "₫", useComma: true, useDecimals: false, useSpace: true, right: true},
	"myr": {symbol: "RM", useComma: false, useDecimals: true, useSpace: false, right: false},
	"mad": {symbol: "DH", useComma: false, useDecimals: true, useSpace: true, right: true},
}

// formatCurrencyGo formats an amount with the appropriate currency symbol and rules
func formatCurrencyGo(amount float64, currency string) string {
	behavior, ok := currencyBehaviors[strings.ToLower(currency)]
	if !ok {
		// Default to USD format
		behavior = currencyBehavior{symbol: "$", useComma: false, useDecimals: true, useSpace: false, right: false}
	}

	absAmount := math.Abs(amount)
	var formattedAmount string

	if behavior.useDecimals {
		if behavior.useComma {
			// European format: 1.234,56
			formattedAmount = formatNumberWithCommaDecimal(absAmount)
		} else {
			// US format: 1,234.56
			formattedAmount = formatNumberWithDotDecimal(absAmount)
		}
	} else {
		// No decimals
		formattedAmount = formatNumberNoDecimal(absAmount)
	}

	// Build final string with symbol placement
	var result string
	if behavior.right {
		if behavior.useSpace {
			result = formattedAmount + " " + behavior.symbol
		} else {
			result = formattedAmount + behavior.symbol
		}
	} else {
		if behavior.useSpace {
			result = behavior.symbol + " " + formattedAmount
		} else {
			result = behavior.symbol + formattedAmount
		}
	}

	return result
}

// formatNumberWithDotDecimal formats a number with dot as decimal separator (US style)
func formatNumberWithDotDecimal(amount float64) string {
	// Format with 2 decimal places
	str := fmt.Sprintf("%.2f", amount)
	parts := strings.Split(str, ".")

	// Add thousand separators
	intPart := parts[0]
	var result strings.Builder
	for i, digit := range intPart {
		if i > 0 && (len(intPart)-i)%3 == 0 {
			result.WriteRune(',')
		}
		result.WriteRune(digit)
	}

	return result.String() + "." + parts[1]
}

// formatNumberWithCommaDecimal formats a number with comma as decimal separator (EU style)
func formatNumberWithCommaDecimal(amount float64) string {
	// Format with 2 decimal places
	str := fmt.Sprintf("%.2f", amount)
	parts := strings.Split(str, ".")

	// Add thousand separators (dots in EU format)
	intPart := parts[0]
	var result strings.Builder
	for i, digit := range intPart {
		if i > 0 && (len(intPart)-i)%3 == 0 {
			result.WriteRune('.')
		}
		result.WriteRune(digit)
	}

	return result.String() + "," + parts[1]
}

// formatNumberNoDecimal formats a number without decimals
func formatNumberNoDecimal(amount float64) string {
	str := fmt.Sprintf("%.0f", amount)

	// Add thousand separators
	var result strings.Builder
	for i, digit := range str {
		if i > 0 && (len(str)-i)%3 == 0 {
			result.WriteRune(',')
		}
		result.WriteRune(digit)
	}

	return result.String()
}

// formatDateHuman formats a date in human-readable format with localized month names
func formatDateHuman(date time.Time, language string) string {
	// Month keys for localization lookup
	monthKeys := []string{
		"months.january", "months.february", "months.march", "months.april",
		"months.may", "months.june", "months.july", "months.august",
		"months.september", "months.october", "months.november", "months.december",
	}

	// Get localized month name (Month() returns 1-12)
	monthName := getLocalizedString(language, monthKeys[date.Month()-1])

	// Format: "2 January 2006"
	return fmt.Sprintf("%d %s %d", date.Day(), monthName, date.Year())
}

// formatTimestampHuman formats a timestamp in human-readable format with localized month names
func formatTimestampHuman(t time.Time, language string) string {
	// Month keys for localization lookup
	monthKeys := []string{
		"months.january", "months.february", "months.march", "months.april",
		"months.may", "months.june", "months.july", "months.august",
		"months.september", "months.october", "months.november", "months.december",
	}

	// Get localized month name (Month() returns 1-12)
	monthName := getLocalizedString(language, monthKeys[t.Month()-1])

	// Get timezone name
	zone, _ := t.Zone()

	// Format: "2 January 2006, 15:04 MST" (using server local time)
	return fmt.Sprintf("%d %s %d, %02d:%02d %s",
		t.Day(), monthName, t.Year(), t.Hour(), t.Minute(), zone)
}

// shortenID returns first 8 characters of a UUID
func shortenID(id string) string {
	if len(id) > 8 {
		return id[:8]
	}
	return id
}

// GenerateReceiptPDF generates a PDF receipt for gain transactions
func (h *Handler) GenerateReceiptPDF(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeJSON(w, http.StatusMethodNotAllowed, ErrorResponse{Error: "Method not allowed"})
		return
	}

	// Extract and validate ID parameter
	id := r.URL.Query().Get("id")
	if id == "" {
		writeJSON(w, http.StatusBadRequest, ErrorResponse{Error: "Missing id parameter"})
		return
	}

	// Fetch all expenses and find the one with matching ID
	expenses, err := h.storage.GetAllExpenses()
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, ErrorResponse{Error: "Failed to retrieve expenses"})
		log.Printf("API ERROR: Failed to retrieve expenses for receipt generation: %v\n", err)
		return
	}

	var expense *storage.Expense
	for i := range expenses {
		if expenses[i].ID == id {
			expense = &expenses[i]
			break
		}
	}

	if expense == nil {
		writeJSON(w, http.StatusNotFound, ErrorResponse{Error: "Transaction not found"})
		log.Printf("API ERROR: Transaction with ID %s not found\n", id)
		return
	}

	// Validate it's a gain (positive amount)
	if expense.Amount <= 0 {
		writeJSON(w, http.StatusBadRequest, ErrorResponse{Error: "Transaction is not a gain. Use voucher endpoint for expenses."})
		log.Printf("API ERROR: Attempted to generate receipt for expense (amount <= 0) with ID %s\n", id)
		return
	}

	// Get user preferences
	language, err := h.storage.GetLanguage()
	if err != nil {
		log.Printf("Warning: Failed to get language preference, defaulting to English: %v\n", err)
		language = "en"
	}

	currency := expense.Currency
	if currency == "" {
		currency, _ = h.storage.GetCurrency()
		if currency == "" {
			currency = "usd"
		}
	}

	// Generate PDF
	pdfBytes, err := buildReceiptPDF(*expense, language, currency)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, ErrorResponse{Error: "Failed to generate PDF"})
		log.Printf("API ERROR: Failed to generate receipt PDF for ID %s: %v\n", id, err)
		return
	}

	// Set headers and stream PDF
	w.Header().Set("Content-Type", "application/pdf")
	w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=receipt-%s.pdf", shortenID(id)))
	w.Write(pdfBytes)

	log.Printf("HTTP: Generated receipt PDF for transaction ID %s\n", id)
}

// GenerateVoucherPDF generates a PDF payment voucher for expense transactions
func (h *Handler) GenerateVoucherPDF(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeJSON(w, http.StatusMethodNotAllowed, ErrorResponse{Error: "Method not allowed"})
		return
	}

	// Extract and validate ID parameter
	id := r.URL.Query().Get("id")
	if id == "" {
		writeJSON(w, http.StatusBadRequest, ErrorResponse{Error: "Missing id parameter"})
		return
	}

	// Fetch all expenses and find the one with matching ID
	expenses, err := h.storage.GetAllExpenses()
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, ErrorResponse{Error: "Failed to retrieve expenses"})
		log.Printf("API ERROR: Failed to retrieve expenses for voucher generation: %v\n", err)
		return
	}

	var expense *storage.Expense
	for i := range expenses {
		if expenses[i].ID == id {
			expense = &expenses[i]
			break
		}
	}

	if expense == nil {
		writeJSON(w, http.StatusNotFound, ErrorResponse{Error: "Transaction not found"})
		log.Printf("API ERROR: Transaction with ID %s not found\n", id)
		return
	}

	// Validate it's an expense (negative amount)
	if expense.Amount >= 0 {
		writeJSON(w, http.StatusBadRequest, ErrorResponse{Error: "Transaction is not an expense. Use receipt endpoint for gains."})
		log.Printf("API ERROR: Attempted to generate voucher for gain (amount >= 0) with ID %s\n", id)
		return
	}

	// Get user preferences
	language, err := h.storage.GetLanguage()
	if err != nil {
		log.Printf("Warning: Failed to get language preference, defaulting to English: %v\n", err)
		language = "en"
	}

	currency := expense.Currency
	if currency == "" {
		currency, _ = h.storage.GetCurrency()
		if currency == "" {
			currency = "usd"
		}
	}

	// Generate PDF
	pdfBytes, err := buildVoucherPDF(*expense, language, currency)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, ErrorResponse{Error: "Failed to generate PDF"})
		log.Printf("API ERROR: Failed to generate voucher PDF for ID %s: %v\n", id, err)
		return
	}

	// Set headers and stream PDF
	w.Header().Set("Content-Type", "application/pdf")
	w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=voucher-%s.pdf", shortenID(id)))
	w.Write(pdfBytes)

	log.Printf("HTTP: Generated voucher PDF for transaction ID %s\n", id)
}

// buildReceiptPDF creates a PDF receipt for a gain transaction
func buildReceiptPDF(expense storage.Expense, language, currency string) ([]byte, error) {
	// Create maroto configuration
	cfg := config.NewBuilder().
		WithPageSize(pagesize.A4).
		WithOrientation(orientation.Vertical).
		WithLeftMargin(10).
		WithTopMargin(15).
		WithRightMargin(10).
		Build()

	m := maroto.New(cfg)

	// Load logo from embedded filesystem
	fs := web.GetTemplates()
	logoBytes, err := fs.ReadFile("templates/pwa/icon-512.png")
	if err != nil {
		log.Printf("Warning: Failed to load logo for receipt: %v\n", err)
	}

	// Logo and Title header
	if logoBytes != nil {
		m.AddRow(25,
			col.New(3), // Left spacing
			col.New(6).Add(
				image.NewFromBytes(logoBytes, extension.Png, props.Rect{
					Center:  true,
					Percent: 60,
				}),
			),
			col.New(3), // Right spacing
		)
		m.AddRow(3) // Small spacing
	}

	// Title
	m.AddRow(12,
		text.NewCol(12, getLocalizedString(language, "receipt.title"),
			props.Text{
				Top:   3,
				Size:  16,
				Style: fontstyle.Bold,
				Align: align.Center,
			}),
	)

	// Spacing
	m.AddRow(5)

	// Received From section
	receivedFromLabel := getLocalizedString(language, "receipt.received_from")
	receivedFromValue := expense.From
	if receivedFromValue == "" {
		receivedFromValue = "-"
	}

	m.AddRow(8,
		col.New(4).Add(
			text.New(receivedFromLabel+":", props.Text{
				Size:  10,
				Style: fontstyle.Bold,
			}),
		),
		col.New(8).Add(
			text.New(receivedFromValue, props.Text{
				Size: 10,
			}),
		),
	)

	// Transaction ID
	transactionIDLabel := getLocalizedString(language, "receipt.transaction_id")
	m.AddRow(8,
		col.New(4).Add(
			text.New(transactionIDLabel+":", props.Text{
				Size:  10,
				Style: fontstyle.Bold,
			}),
		),
		col.New(8).Add(
			text.New(shortenID(expense.ID), props.Text{
				Size: 10,
			}),
		),
	)

	// Date
	dateLabel := getLocalizedString(language, "common.date")
	m.AddRow(8,
		col.New(4).Add(
			text.New(dateLabel+":", props.Text{
				Size:  10,
				Style: fontstyle.Bold,
			}),
		),
		col.New(8).Add(
			text.New(formatDateHuman(expense.Date, language), props.Text{
				Size: 10,
			}),
		),
	)

	// Spacing
	m.AddRow(5)

	// Transaction Details header
	detailsLabel := getLocalizedString(language, "receipt.details")
	m.AddRow(10,
		text.NewCol(12, detailsLabel,
			props.Text{
				Top:   3,
				Size:  12,
				Style: fontstyle.Bold,
				Align: align.Center,
			}),
	)

	// Spacing
	m.AddRow(3)

	// Description
	descriptionLabel := getLocalizedString(language, "common.description")
	m.AddRow(8,
		col.New(4).Add(
			text.New(descriptionLabel+":", props.Text{
				Size:  10,
				Style: fontstyle.Bold,
			}),
		),
		col.New(8).Add(
			text.New(expense.Description, props.Text{
				Size: 10,
			}),
		),
	)

	// Category
	categoryLabel := getLocalizedString(language, "common.category")
	m.AddRow(8,
		col.New(4).Add(
			text.New(categoryLabel+":", props.Text{
				Size:  10,
				Style: fontstyle.Bold,
			}),
		),
		col.New(8).Add(
			text.New(expense.Category, props.Text{
				Size: 10,
			}),
		),
	)

	// Payment Method
	method := expense.Method
	if method == "" {
		method = "cash"
	}
	methodLabel := getLocalizedString(language, "common.method")
	localizedMethod := getLocalizedString(language, "method."+method)
	m.AddRow(8,
		col.New(4).Add(
			text.New(methodLabel+":", props.Text{
				Size:  10,
				Style: fontstyle.Bold,
			}),
		),
		col.New(8).Add(
			text.New(localizedMethod, props.Text{
				Size: 10,
			}),
		),
	)

	// Amount
	amountLabel := getLocalizedString(language, "document.amount")
	formattedAmount := formatCurrencyGo(expense.Amount, currency)
	m.AddRow(8,
		col.New(4).Add(
			text.New(amountLabel+":", props.Text{
				Size:  10,
				Style: fontstyle.Bold,
			}),
		),
		col.New(8).Add(
			text.New(formattedAmount, props.Text{
				Size:  11,
				Style: fontstyle.Bold,
			}),
		),
	)

	// Spacing
	m.AddRow(10)

	// Footer
	generatedByLabel := getLocalizedString(language, "receipt.generated_by")
	generatedOnLabel := getLocalizedString(language, "receipt.generated_on")
	currentTime := formatTimestampHuman(time.Now(), language)

	m.AddRow(8,
		text.NewCol(12, generatedByLabel,
			props.Text{
				Size:  9,
				Align: align.Center,
			}),
	)

	m.AddRow(8,
		text.NewCol(12, generatedOnLabel+": "+currentTime,
			props.Text{
				Size:  8,
				Align: align.Center,
			}),
	)

	// Generate PDF bytes
	doc, err := m.Generate()
	if err != nil {
		return nil, err
	}

	return doc.GetBytes(), nil
}

// buildVoucherPDF creates a PDF payment voucher for an expense transaction
func buildVoucherPDF(expense storage.Expense, language, currency string) ([]byte, error) {
	// Create maroto configuration
	cfg := config.NewBuilder().
		WithPageSize(pagesize.A4).
		WithOrientation(orientation.Vertical).
		WithLeftMargin(10).
		WithTopMargin(15).
		WithRightMargin(10).
		Build()

	m := maroto.New(cfg)

	// Load logo from embedded filesystem
	fs := web.GetTemplates()
	logoBytes, err := fs.ReadFile("templates/pwa/icon-512.png")
	if err != nil {
		log.Printf("Warning: Failed to load logo for voucher: %v\n", err)
	}

	// Logo and Title header
	if logoBytes != nil {
		m.AddRow(25,
			col.New(3), // Left spacing
			col.New(6).Add(
				image.NewFromBytes(logoBytes, extension.Png, props.Rect{
					Center:  true,
					Percent: 60,
				}),
			),
			col.New(3), // Right spacing
		)
		m.AddRow(3) // Small spacing
	}

	// Title
	m.AddRow(12,
		text.NewCol(12, getLocalizedString(language, "voucher.title"),
			props.Text{
				Top:   3,
				Size:  16,
				Style: fontstyle.Bold,
				Align: align.Center,
			}),
	)

	// Spacing
	m.AddRow(5)

	// Paid To section
	paidToLabel := getLocalizedString(language, "voucher.paid_to")
	paidToValue := expense.To
	if paidToValue == "" {
		paidToValue = "-"
	}

	m.AddRow(8,
		col.New(4).Add(
			text.New(paidToLabel+":", props.Text{
				Size:  10,
				Style: fontstyle.Bold,
			}),
		),
		col.New(8).Add(
			text.New(paidToValue, props.Text{
				Size: 10,
			}),
		),
	)

	// Voucher ID
	voucherIDLabel := getLocalizedString(language, "voucher.voucher_id")
	m.AddRow(8,
		col.New(4).Add(
			text.New(voucherIDLabel+":", props.Text{
				Size:  10,
				Style: fontstyle.Bold,
			}),
		),
		col.New(8).Add(
			text.New(shortenID(expense.ID), props.Text{
				Size: 10,
			}),
		),
	)

	// Date
	dateLabel := getLocalizedString(language, "common.date")
	m.AddRow(8,
		col.New(4).Add(
			text.New(dateLabel+":", props.Text{
				Size:  10,
				Style: fontstyle.Bold,
			}),
		),
		col.New(8).Add(
			text.New(formatDateHuman(expense.Date, language), props.Text{
				Size: 10,
			}),
		),
	)

	// Spacing
	m.AddRow(5)

	// Expense Details header
	detailsLabel := getLocalizedString(language, "voucher.details")
	m.AddRow(10,
		text.NewCol(12, detailsLabel,
			props.Text{
				Top:   3,
				Size:  12,
				Style: fontstyle.Bold,
				Align: align.Center,
			}),
	)

	// Spacing
	m.AddRow(3)

	// Description
	descriptionLabel := getLocalizedString(language, "common.description")
	m.AddRow(8,
		col.New(4).Add(
			text.New(descriptionLabel+":", props.Text{
				Size:  10,
				Style: fontstyle.Bold,
			}),
		),
		col.New(8).Add(
			text.New(expense.Description, props.Text{
				Size: 10,
			}),
		),
	)

	// Category
	categoryLabel := getLocalizedString(language, "common.category")
	m.AddRow(8,
		col.New(4).Add(
			text.New(categoryLabel+":", props.Text{
				Size:  10,
				Style: fontstyle.Bold,
			}),
		),
		col.New(8).Add(
			text.New(expense.Category, props.Text{
				Size: 10,
			}),
		),
	)

	// Payment Method
	method := expense.Method
	if method == "" {
		method = "cash"
	}
	methodLabel := getLocalizedString(language, "common.method")
	localizedMethod := getLocalizedString(language, "method."+method)
	m.AddRow(8,
		col.New(4).Add(
			text.New(methodLabel+":", props.Text{
				Size:  10,
				Style: fontstyle.Bold,
			}),
		),
		col.New(8).Add(
			text.New(localizedMethod, props.Text{
				Size: 10,
			}),
		),
	)

	// Amount (absolute value for expenses)
	amountLabel := getLocalizedString(language, "document.amount")
	formattedAmount := formatCurrencyGo(math.Abs(expense.Amount), currency)
	m.AddRow(8,
		col.New(4).Add(
			text.New(amountLabel+":", props.Text{
				Size:  10,
				Style: fontstyle.Bold,
			}),
		),
		col.New(8).Add(
			text.New(formattedAmount, props.Text{
				Size:  11,
				Style: fontstyle.Bold,
			}),
		),
	)

	// Spacing
	m.AddRow(10)

	// Footer
	forInternalLabel := getLocalizedString(language, "voucher.for_internal")
	generatedOnLabel := getLocalizedString(language, "receipt.generated_on")
	currentTime := formatTimestampHuman(time.Now(), language)

	m.AddRow(8,
		text.NewCol(12, forInternalLabel,
			props.Text{
				Size:  9,
				Align: align.Center,
			}),
	)

	m.AddRow(8,
		text.NewCol(12, generatedOnLabel+": "+currentTime,
			props.Text{
				Size:  8,
				Align: align.Center,
			}),
	)

	// Generate PDF bytes
	doc, err := m.Generate()
	if err != nil {
		return nil, err
	}

	return doc.GetBytes(), nil
}
