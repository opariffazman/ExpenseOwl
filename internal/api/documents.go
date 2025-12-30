package api

import (
	"encoding/json"
	"fmt"
	"log"
	"math"
	"net/http"
	"strings"
	"time"

	"github.com/johnfercher/maroto/v2"
	"github.com/johnfercher/maroto/v2/pkg/components/col"
	"github.com/johnfercher/maroto/v2/pkg/components/image"
	"github.com/johnfercher/maroto/v2/pkg/components/line"
	"github.com/johnfercher/maroto/v2/pkg/components/row"
	"github.com/johnfercher/maroto/v2/pkg/components/text"
	"github.com/johnfercher/maroto/v2/pkg/config"
	"github.com/johnfercher/maroto/v2/pkg/consts/align"
	"github.com/johnfercher/maroto/v2/pkg/consts/border"
	"github.com/johnfercher/maroto/v2/pkg/consts/extension"
	"github.com/johnfercher/maroto/v2/pkg/consts/fontstyle"
	"github.com/johnfercher/maroto/v2/pkg/consts/pagesize"
	"github.com/johnfercher/maroto/v2/pkg/consts/orientation"
	"github.com/johnfercher/maroto/v2/pkg/core"
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

// addLetterheadHeader adds the standard PBAKTH letterhead header to a PDF document
func addLetterheadHeader(m core.Maroto) {
	// Load header logo from embedded filesystem
	fs := web.GetTemplates()
	logoBytes, err := fs.ReadFile("templates/pwa/header.png")
	if err != nil {
		log.Printf("Warning: Failed to load header logo: %v\n", err)
	}

	// Header: Logo
	if logoBytes != nil {
		m.AddRow(30,
			col.New(12).Add(
				image.NewFromBytes(logoBytes, extension.Png, props.Rect{
					Center:  true,
					Percent: 100,
				}),
			),
		)
		m.AddRow(2) // Small spacing
	}

	// Horizontal line separator after header
	m.AddRow(5,
		line.NewCol(12),
	)

	// Spacing
	m.AddRow(3)
}

// addLetterheadFooter registers the standard PBAKTH letterhead footer to appear at the bottom of each page
// firstMessageKey is the localization key for the first message (e.g., "receipt.generated_by" or "voucher.for_internal")
func addLetterheadFooter(m core.Maroto, language string, firstMessageKey string) {
	// Pre-calculate localized strings
	firstMessage := getLocalizedString(language, firstMessageKey)
	generatedOnLabel := getLocalizedString(language, "receipt.generated_on")
	currentTime := formatTimestampHuman(time.Now(), language)

	// Register footer to appear at bottom of every page
	m.RegisterFooter(
		row.New(5).Add(
			col.New(12).Add(
				line.New(),
			),
		),
		row.New(8).Add(
			text.NewCol(12, "(NO. PENDAFTARAN: PPM-008-14-14042009)",
				props.Text{
					Size:  9,
					Style: fontstyle.Bold,
					Align: align.Center,
				}),
		),
		row.New(8).Add(
			text.NewCol(12, "TINGKAT 4, BANGUNAN TH SELBORN, JALAN TUN RAZAK, 50300 KUALA LUMPUR.",
				props.Text{
					Size:  8,
					Align: align.Center,
				}),
		),
		row.New(8).Add(
			text.NewCol(12, firstMessage,
				props.Text{
					Size:  9,
					Align: align.Center,
					Top:   3,
				}),
		),
		row.New(8).Add(
			text.NewCol(12, generatedOnLabel+": "+currentTime,
				props.Text{
					Size:  8,
					Align: align.Center,
				}),
		),
	)
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
		WithBottomMargin(10).
		Build()

	m := maroto.New(cfg)

	// Add letterhead header
	addLetterheadHeader(m)

	// Add letterhead footer (will appear at page bottom)
	addLetterheadFooter(m, language, "receipt.generated_by")

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

	// Note (if exists)
	if expense.Note != "" {
		noteLabel := getLocalizedString(language, "common.note")
		m.AddRow(8,
			col.New(4).Add(
				text.New(noteLabel+":", props.Text{
					Size:  10,
					Style: fontstyle.Bold,
				}),
			),
			col.New(8).Add(
				text.New(expense.Note, props.Text{
					Size: 10,
				}),
			),
		)
	}

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
		WithBottomMargin(10).
		Build()

	m := maroto.New(cfg)

	// Add letterhead header
	addLetterheadHeader(m)

	// Add letterhead footer (will appear at page bottom)
	addLetterheadFooter(m, language, "voucher.for_internal")

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

	// Note (if exists)
	if expense.Note != "" {
		noteLabel := getLocalizedString(language, "common.note")
		m.AddRow(8,
			col.New(4).Add(
				text.New(noteLabel+":", props.Text{
					Size:  10,
					Style: fontstyle.Bold,
				}),
			),
			col.New(8).Add(
				text.New(expense.Note, props.Text{
					Size: 10,
				}),
			),
		)
	}

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

	// Generate PDF bytes
	doc, err := m.Generate()
	if err != nil {
		return nil, err
	}

	return doc.GetBytes(), nil
}

// GenerateReportPDF generates a PDF report for expenses or gains based on period
func (h *Handler) GenerateReportPDF(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeJSON(w, http.StatusMethodNotAllowed, ErrorResponse{Error: "Method not allowed"})
		return
	}

	// Extract query parameters
	transactionType := r.URL.Query().Get("type")   // "expenses" or "gains"
	period := r.URL.Query().Get("period")          // "daily", "monthly", "yearly"
	yearStr := r.URL.Query().Get("year")
	monthStr := r.URL.Query().Get("month")

	// Validate transaction type
	if transactionType != "expenses" && transactionType != "gains" {
		writeJSON(w, http.StatusBadRequest, ErrorResponse{Error: "Invalid type parameter. Must be 'expenses' or 'gains'"})
		return
	}

	// Validate period
	if period != "daily" && period != "monthly" && period != "yearly" {
		writeJSON(w, http.StatusBadRequest, ErrorResponse{Error: "Invalid period parameter. Must be 'daily', 'monthly', or 'yearly'"})
		return
	}

	// Fetch all expenses
	expenses, err := h.storage.GetAllExpenses()
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, ErrorResponse{Error: "Failed to retrieve expenses"})
		log.Printf("API ERROR: Failed to retrieve expenses for report generation: %v\n", err)
		return
	}

	// Get user preferences
	language, err := h.storage.GetLanguage()
	if err != nil {
		log.Printf("Warning: Failed to get language preference, defaulting to English: %v\n", err)
		language = "en"
	}

	currency, err := h.storage.GetCurrency()
	if err != nil || currency == "" {
		currency = "usd"
	}

	// Filter expenses by type (expenses or gains)
	var filteredExpenses []storage.Expense
	for _, exp := range expenses {
		if transactionType == "gains" && exp.Amount > 0 {
			filteredExpenses = append(filteredExpenses, exp)
		} else if transactionType == "expenses" && exp.Amount < 0 {
			filteredExpenses = append(filteredExpenses, exp)
		}
	}

	// Filter by date range if year/month provided
	if yearStr != "" && monthStr != "" {
		// Parse year and month
		var year, month int
		fmt.Sscanf(yearStr, "%d", &year)
		fmt.Sscanf(monthStr, "%d", &month)

		// Get start date setting
		startDate, err := h.storage.GetStartDate()
		if err != nil {
			startDate = 1
		}

		// Calculate date range based on start date
		var startTime, endTime time.Time
		if startDate == 1 {
			// Normal month: 1st to last day
			startTime = time.Date(year, time.Month(month), 1, 0, 0, 0, 0, time.UTC)
			endTime = time.Date(year, time.Month(month+1), 0, 23, 59, 59, 0, time.UTC)
		} else {
			// Custom start date
			startTime = time.Date(year, time.Month(month), startDate, 0, 0, 0, 0, time.UTC)
			if month == 12 {
				endTime = time.Date(year+1, 1, startDate-1, 23, 59, 59, 0, time.UTC)
			} else {
				endTime = time.Date(year, time.Month(month+1), startDate-1, 23, 59, 59, 0, time.UTC)
			}
		}

		// Filter expenses by date range
		var dateFilteredExpenses []storage.Expense
		for _, exp := range filteredExpenses {
			if (exp.Date.Equal(startTime) || exp.Date.After(startTime)) && (exp.Date.Equal(endTime) || exp.Date.Before(endTime)) {
				dateFilteredExpenses = append(dateFilteredExpenses, exp)
			}
		}
		filteredExpenses = dateFilteredExpenses
	}

	// Generate PDF
	pdfBytes, err := buildReportPDF(filteredExpenses, transactionType, period, language, currency)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, ErrorResponse{Error: "Failed to generate PDF"})
		log.Printf("API ERROR: Failed to generate report PDF: %v\n", err)
		return
	}

	// Set headers and stream PDF
	filename := fmt.Sprintf("%s-report-%s.pdf", transactionType, period)
	w.Header().Set("Content-Type", "application/pdf")
	w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=%s", filename))
	w.Write(pdfBytes)

	log.Printf("HTTP: Generated %s report PDF for period %s\n", transactionType, period)
}

// buildReportPDF builds a PDF document containing a table of transactions
func buildReportPDF(expenses []storage.Expense, transactionType, period, language, currency string) ([]byte, error) {
	// Create maroto instance
	cfg := config.NewBuilder().
		WithPageSize(pagesize.A4).
		WithOrientation(orientation.Vertical).
		WithLeftMargin(10).
		WithTopMargin(15).
		WithRightMargin(10).
		WithBottomMargin(10).
		Build()

	m := maroto.New(cfg)

	// Get localized strings
	titleLabel := getLocalizedString(language, "report.title")
	if titleLabel == "report.title" {
		titleLabel = "Transaction Report"
	}

	typeLabel := ""
	if transactionType == "expenses" {
		typeLabel = getLocalizedString(language, "dashboard.expenses")
		if typeLabel == "dashboard.expenses" {
			typeLabel = "Expenses"
		}
	} else {
		typeLabel = getLocalizedString(language, "dashboard.income")
		if typeLabel == "dashboard.income" {
			typeLabel = "Gains"
		}
	}

	periodLabel := getLocalizedString(language, "report."+period)
	if periodLabel == "report."+period {
		periodLabel = strings.Title(period)
	}

	descriptionLabel := getLocalizedString(language, "common.description")
	partyLabel := getLocalizedString(language, "common.party")
	categoryLabel := getLocalizedString(language, "common.category")
	amountLabel := getLocalizedString(language, "common.amount")
	dateLabel := getLocalizedString(language, "common.date")
	totalLabel := getLocalizedString(language, "dashboard.total")
	if totalLabel == "dashboard.total" {
		totalLabel = "Total:"
	}

	// Add letterhead header
	addLetterheadHeader(m)

	// Add letterhead footer (will appear at page bottom)
	addLetterheadFooter(m, language, "receipt.generated_by")

	// Title
	m.AddRow(12,
		text.NewCol(12, titleLabel,
			props.Text{
				Top:   3,
				Size:  16,
				Style: fontstyle.Bold,
				Align: align.Center,
			}),
	)

	// Add report type and period
	m.AddRow(10,
		text.NewCol(12, fmt.Sprintf("%s - %s", typeLabel, periodLabel),
			props.Text{
				Size:  12,
				Align: align.Center,
			}),
	)

	// Spacing
	m.AddRow(5)

	// Add table header
	m.AddRow(8,
		text.NewCol(3, dateLabel,
			props.Text{
				Size:  10,
				Style: fontstyle.Bold,
				Align: align.Left,
			}),
		text.NewCol(3, descriptionLabel,
			props.Text{
				Size:  10,
				Style: fontstyle.Bold,
				Align: align.Left,
			}),
		text.NewCol(2, partyLabel,
			props.Text{
				Size:  10,
				Style: fontstyle.Bold,
				Align: align.Left,
			}),
		text.NewCol(2, categoryLabel,
			props.Text{
				Size:  10,
				Style: fontstyle.Bold,
				Align: align.Left,
			}),
		text.NewCol(2, amountLabel,
			props.Text{
				Size:  10,
				Style: fontstyle.Bold,
				Align: align.Right,
			}),
	)

	// Add line separator
	m.AddRow(2,
		line.NewCol(12),
	)

	// Calculate total
	var total float64

	// Add table rows
	for _, exp := range expenses {
		party := exp.To
		if exp.Amount > 0 {
			party = exp.From
		}

		total += math.Abs(exp.Amount)

		dateStr := exp.Date.Format("2006-01-02")

		m.AddRow(7,
			text.NewCol(3, dateStr,
				props.Text{
					Size:  9,
					Align: align.Left,
				}),
			text.NewCol(3, exp.Description,
				props.Text{
					Size:  9,
					Align: align.Left,
				}),
			text.NewCol(2, party,
				props.Text{
					Size:  9,
					Align: align.Left,
				}),
			text.NewCol(2, exp.Category,
				props.Text{
					Size:  9,
					Align: align.Left,
				}),
			text.NewCol(2, formatCurrencyGo(math.Abs(exp.Amount), currency),
				props.Text{
					Size:  9,
					Align: align.Right,
				}),
		)
	}

	// Add total
	m.AddRow(2,
		line.NewCol(12),
	)

	m.AddRow(10,
		text.NewCol(10, totalLabel,
			props.Text{
				Size:  11,
				Style: fontstyle.Bold,
				Align: align.Right,
			}),
		text.NewCol(2, formatCurrencyGo(total, currency),
			props.Text{
				Size:  11,
				Style: fontstyle.Bold,
				Align: align.Right,
			}),
	)

	// Generate PDF bytes
	doc, err := m.Generate()
	if err != nil {
		return nil, err
	}

	return doc.GetBytes(), nil
}

// GenerateStatementPDF generates a trial balance PDF statement
func (h *Handler) GenerateStatementPDF(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeJSON(w, http.StatusMethodNotAllowed, ErrorResponse{Error: "Method not allowed"})
		return
	}

	// Parse request body
	var requestData struct {
		StartDate *string            `json:"startDate"`
		EndDate   *string            `json:"endDate"`
		Expenses  []storage.Expense  `json:"expenses"`
	}

	if err := json.NewDecoder(r.Body).Decode(&requestData); err != nil {
		writeJSON(w, http.StatusBadRequest, ErrorResponse{Error: "Invalid request body"})
		log.Printf("API ERROR: Failed to decode statement request: %v\n", err)
		return
	}

	// Parse dates if provided
	var startDate, endDate *time.Time
	if requestData.StartDate != nil && *requestData.StartDate != "" {
		parsed, err := time.Parse(time.RFC3339, *requestData.StartDate)
		if err != nil {
			log.Printf("Warning: Failed to parse start date: %v\n", err)
		} else {
			startDate = &parsed
		}
	}
	if requestData.EndDate != nil && *requestData.EndDate != "" {
		parsed, err := time.Parse(time.RFC3339, *requestData.EndDate)
		if err != nil {
			log.Printf("Warning: Failed to parse end date: %v\n", err)
		} else {
			endDate = &parsed
		}
	}

	// Get opening balance
	openingBalance, err := h.storage.GetOpeningBalance()
	if err != nil {
		log.Printf("Warning: Failed to get opening balance, using 0: %v\n", err)
		openingBalance = 0
	}

	// Get user preferences
	language, err := h.storage.GetLanguage()
	if err != nil {
		log.Printf("Warning: Failed to get language preference, defaulting to English: %v\n", err)
		language = "en"
	}

	currency, err := h.storage.GetCurrency()
	if err != nil || currency == "" {
		currency = "usd"
	}

	// Generate PDF
	pdfBytes, err := buildStatementPDF(requestData.Expenses, startDate, endDate, openingBalance, language, currency)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, ErrorResponse{Error: "Failed to generate statement PDF"})
		log.Printf("API ERROR: Failed to generate statement PDF: %v\n", err)
		return
	}

	// Set headers and stream PDF
	filename := fmt.Sprintf("statement-%s.pdf", time.Now().Format("2006-01-02"))
	w.Header().Set("Content-Type", "application/pdf")
	w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=%s", filename))
	w.Write(pdfBytes)

	log.Printf("HTTP: Generated statement PDF\n")
}

// buildStatementPDF creates a trial balance PDF with debit and credit columns
func buildStatementPDF(expenses []storage.Expense, startDate, endDate *time.Time, openingBalance float64, language, currency string) ([]byte, error) {
	// Create maroto configuration
	cfg := config.NewBuilder().
		WithPageSize(pagesize.A4).
		WithOrientation(orientation.Vertical).
		WithLeftMargin(10).
		WithTopMargin(15).
		WithRightMargin(10).
		WithBottomMargin(10).
		Build()

	m := maroto.New(cfg)

	// Add letterhead header
	addLetterheadHeader(m)

	// Add letterhead footer
	addLetterheadFooter(m, language, "receipt.generated_by")

	// Get localized strings
	statementTitle := getLocalizedString(language, "statement.title")
	if statementTitle == "statement.title" {
		statementTitle = "PENYATA KEWANGAN"
	}

	accountBalanceLabel := getLocalizedString(language, "statement.opening_balance")
	if accountBalanceLabel == "statement.opening_balance" {
		accountBalanceLabel = "Baki Akaun"
	}

	debitLabel := getLocalizedString(language, "statement.debit")
	if debitLabel == "statement.debit" {
		debitLabel = "PENGELUARAN"
	}

	creditLabel := getLocalizedString(language, "statement.credit")
	if creditLabel == "statement.credit" {
		creditLabel = "PENERIMAAN"
	}

	categoryLabel := getLocalizedString(language, "common.category")
	if categoryLabel == "common.category" {
		categoryLabel = "Category"
	}

	amountLabel := getLocalizedString(language, "common.amount")
	if amountLabel == "common.amount" {
		amountLabel = "Amount"
	}

	totalLabel := getLocalizedString(language, "statement.total")
	if totalLabel == "statement.total" {
		totalLabel = "JUMLAH"
	}

	fiscalYearLabel := getLocalizedString(language, "statement.fiscal_year")
	if fiscalYearLabel == "statement.fiscal_year" {
		fiscalYearLabel = "Tahun Fiskal"
	}

	// Get January and December localized names
	januaryLabel := getLocalizedString(language, "months.january")
	if januaryLabel == "months.january" {
		januaryLabel = "January"
	}
	decemberLabel := getLocalizedString(language, "months.december")
	if decemberLabel == "months.december" {
		decemberLabel = "December"
	}

	// Determine fiscal year from dates or current year
	var fiscalYear int
	if startDate != nil {
		fiscalYear = startDate.Year()
	} else if endDate != nil {
		fiscalYear = endDate.Year()
	} else {
		fiscalYear = time.Now().Year()
	}

	// Title
	m.AddRow(12,
		text.NewCol(12, statementTitle,
			props.Text{
				Top:   3,
				Size:  16,
				Style: fontstyle.Bold,
				Align: align.Center,
			}),
	)

	// Add fiscal year subtitle (01 Jan to 31 Dec YYYY)
	fiscalYearStr := fmt.Sprintf("%s: 01 %s - 31 %s %d", fiscalYearLabel, januaryLabel, decemberLabel, fiscalYear)
	m.AddRow(10,
		text.NewCol(12, fiscalYearStr,
			props.Text{
				Size:  10,
				Align: align.Center,
			}),
	)

	// Spacing
	m.AddRow(5)

	// Calculate category totals
	debitMap := make(map[string]float64)  // Expenses (negative amounts)
	creditMap := make(map[string]float64) // Gains (positive amounts)

	for _, exp := range expenses {
		category := exp.Category
		if category == "" {
			category = "Uncategorized"
		}

		if exp.Amount < 0 {
			// Debit (expense)
			debitMap[category] += math.Abs(exp.Amount)
		} else if exp.Amount > 0 {
			// Credit (gain)
			creditMap[category] += exp.Amount
		}
	}

	// Convert maps to sorted slices
	type CategoryTotal struct {
		Category string
		Amount   float64
	}

	var debits []CategoryTotal
	for cat, amt := range debitMap {
		debits = append(debits, CategoryTotal{Category: cat, Amount: amt})
	}

	var credits []CategoryTotal
	for cat, amt := range creditMap {
		credits = append(credits, CategoryTotal{Category: cat, Amount: amt})
	}

	// Calculate totals
	totalExpenses := float64(0)
	for _, d := range debits {
		totalExpenses += d.Amount
	}

	totalGains := float64(0)
	for _, c := range credits {
		totalGains += c.Amount
	}

	// Closing balance (Opening Balance + Gains - Expenses)
	closingBalance := openingBalance + totalGains - totalExpenses

	// Total debits includes expenses + closing balance
	totalDebits := totalExpenses + closingBalance

	// Total credits includes opening balance + gains
	totalCredits := openingBalance + totalGains

	// Define border props for table cells - consistent thickness
	borderThickness := 0.5

	// Header borders (all sides)
	headerCreditBorder := props.Cell{
		BorderType:      border.Left | border.Top | border.Bottom | border.Right,
		BorderThickness: borderThickness,
	}
	headerDebitBorder := props.Cell{
		BorderType:      border.Top | border.Bottom | border.Right,
		BorderThickness: borderThickness,
	}

	// Content borders - CREDIT section (left outer border + right middle separator)
	creditContentFirstBorder := props.Cell{
		BorderType:      border.Left | border.Top | border.Right,
		BorderThickness: borderThickness,
	}
	creditContentBorder := props.Cell{
		BorderType:      border.Left | border.Right,
		BorderThickness: borderThickness,
	}
	creditContentLastBorder := props.Cell{
		BorderType:      border.Left | border.Bottom | border.Right,
		BorderThickness: borderThickness,
	}

	// Content borders - DEBIT section (left middle separator + right outer border)
	debitContentFirstBorder := props.Cell{
		BorderType:      border.Left | border.Top | border.Right,
		BorderThickness: borderThickness,
	}
	debitContentBorder := props.Cell{
		BorderType:      border.Left | border.Right,
		BorderThickness: borderThickness,
	}
	debitContentLastBorder := props.Cell{
		BorderType:      border.Left | border.Bottom | border.Right,
		BorderThickness: borderThickness,
	}

	// Total row borders
	totalCreditBorder := props.Cell{
		BorderType:      border.Left | border.Top | border.Bottom | border.Right,
		BorderThickness: borderThickness,
	}
	totalDebitBorder := props.Cell{
		BorderType:      border.Left | border.Top | border.Bottom | border.Right,
		BorderThickness: borderThickness,
	}

	// Add table header with borders (CREDIT on left, DEBIT on right)
	m.AddRow(10,
		col.New(6).Add(
			text.New(creditLabel, props.Text{
				Top:   2,
				Left:  3,
				Right: 3,
				Size:  12,
				Style: fontstyle.Bold,
				Align: align.Center,
			}),
		).WithStyle(&headerCreditBorder),
		col.New(6).Add(
			text.New(debitLabel, props.Text{
				Top:   2,
				Left:  3,
				Right: 3,
				Size:  12,
				Style: fontstyle.Bold,
				Align: align.Center,
			}),
		).WithStyle(&headerDebitBorder),
	)

	// First row: Opening balance on CREDIT side (BOLD) - now on LEFT
	openingBalanceText := fmt.Sprintf("%s - %s", accountBalanceLabel, formatCurrencyGo(openingBalance, currency))
	m.AddRow(8,
		col.New(6).Add(
			text.New(openingBalanceText, props.Text{
				Top:   2,
				Left:  3,
				Right: 3,
				Size:  10,
				Style: fontstyle.Bold,
				Align: align.Left,
			}),
		).WithStyle(&creditContentFirstBorder),
		col.New(6).Add(
			text.New("", props.Text{
				Size: 9,
			}),
		).WithStyle(&debitContentFirstBorder),
	)

	// Add category rows (CREDIT on left, DEBIT on right)
	maxRows := len(debits)
	if len(credits) > maxRows {
		maxRows = len(credits)
	}

	for i := 0; i < maxRows; i++ {
		debitText := ""
		creditText := ""

		if i < len(debits) {
			debitText = fmt.Sprintf("%s - %s", debits[i].Category, formatCurrencyGo(debits[i].Amount, currency))
		}

		if i < len(credits) {
			creditText = fmt.Sprintf("%s - %s", credits[i].Category, formatCurrencyGo(credits[i].Amount, currency))
		}

		m.AddRow(7,
			col.New(6).Add(
				text.New(creditText, props.Text{
					Top:   1,
					Left:  3,
					Right: 3,
					Size:  9,
					Align: align.Left,
				}),
			).WithStyle(&creditContentBorder),
			col.New(6).Add(
				text.New(debitText, props.Text{
					Top:   1,
					Left:  3,
					Right: 3,
					Size:  9,
					Align: align.Left,
				}),
			).WithStyle(&debitContentBorder),
		)
	}

	// Subtotal row: Sum of categories only (excluding account balances)
	creditSubtotalText := fmt.Sprintf("%s - %s", totalLabel, formatCurrencyGo(totalGains, currency))
	debitSubtotalText := fmt.Sprintf("%s - %s", totalLabel, formatCurrencyGo(totalExpenses, currency))
	m.AddRow(8,
		col.New(6).Add(
			text.New(creditSubtotalText, props.Text{
				Top:   2,
				Left:  3,
				Right: 3,
				Size:  10,
				Style: fontstyle.Bold,
				Align: align.Left,
			}),
		).WithStyle(&creditContentBorder),
		col.New(6).Add(
			text.New(debitSubtotalText, props.Text{
				Top:   2,
				Left:  3,
				Right: 3,
				Size:  10,
				Style: fontstyle.Bold,
				Align: align.Left,
			}),
		).WithStyle(&debitContentBorder),
	)

	// Last row: Closing balance on DEBIT side (BOLD) - now on RIGHT
	closingBalanceText := fmt.Sprintf("%s - %s", accountBalanceLabel, formatCurrencyGo(closingBalance, currency))
	m.AddRow(8,
		col.New(6).Add(
			text.New("", props.Text{
				Size: 9,
			}),
		).WithStyle(&creditContentLastBorder),
		col.New(6).Add(
			text.New(closingBalanceText, props.Text{
				Top:   2,
				Left:  3,
				Right: 3,
				Size:  10,
				Style: fontstyle.Bold,
				Align: align.Left,
			}),
		).WithStyle(&debitContentLastBorder),
	)

	// Add final totals row (CREDIT on left, DEBIT on right)
	creditTotalText := fmt.Sprintf("%s - %s", totalLabel, formatCurrencyGo(totalCredits, currency))
	debitTotalText := fmt.Sprintf("%s - %s", totalLabel, formatCurrencyGo(totalDebits, currency))
	m.AddRow(12,
		col.New(6).Add(
			text.New(creditTotalText, props.Text{
				Top:   3,
				Left:  3,
				Right: 3,
				Size:  11,
				Style: fontstyle.Bold,
				Align: align.Left,
			}),
		).WithStyle(&totalCreditBorder),
		col.New(6).Add(
			text.New(debitTotalText, props.Text{
				Top:   3,
				Left:  3,
				Right: 3,
				Size:  11,
				Style: fontstyle.Bold,
				Align: align.Left,
			}),
		).WithStyle(&totalDebitBorder),
	)

	// Generate PDF bytes
	doc, err := m.Generate()
	if err != nil {
		return nil, err
	}

	return doc.GetBytes(), nil
}
