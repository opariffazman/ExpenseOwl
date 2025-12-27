package api

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strconv"
	"time"

	"github.com/tanq16/expenseowl/internal/storage"
	"github.com/tanq16/expenseowl/internal/web"
)

// Handler holds the storage interface
type Handler struct {
	storage storage.Storage
}

// NewHandler creates a new API handler
func NewHandler(s storage.Storage) *Handler {
	return &Handler{
		storage: s,
	}
}

// ErrorResponse is a generic JSON error response
type ErrorResponse struct {
	Error string `json:"error"`
}

// writeJSON is a helper to write JSON responses
func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if v != nil {
		json.NewEncoder(w).Encode(v)
	}
}

// ------------------------------------------------------------
// Config Handlers
// ------------------------------------------------------------

func (h *Handler) GetConfig(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeJSON(w, http.StatusMethodNotAllowed, ErrorResponse{Error: "Method not allowed"})
		return
	}
	config, err := h.storage.GetConfig()
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, ErrorResponse{Error: "Failed to get config"})
		log.Printf("API ERROR: Failed to get config: %v\n", err)
		return
	}
	writeJSON(w, http.StatusOK, config)
}

func (h *Handler) GetCategories(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeJSON(w, http.StatusMethodNotAllowed, ErrorResponse{Error: "Method not allowed"})
		return
	}
	categories, err := h.storage.GetCategories()
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, ErrorResponse{Error: "Failed to get categories"})
		log.Printf("API ERROR: Failed to get categories: %v\n", err)
		return
	}
	writeJSON(w, http.StatusOK, categories)
}

func (h *Handler) UpdateCategories(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPut {
		writeJSON(w, http.StatusMethodNotAllowed, ErrorResponse{Error: "Method not allowed"})
		return
	}
	var categories []string
	if err := json.NewDecoder(r.Body).Decode(&categories); err != nil {
		writeJSON(w, http.StatusBadRequest, ErrorResponse{Error: "Invalid request body"})
		return
	}
	var sanitizedCategories []string
	for _, category := range categories {
		sanitized, err := storage.ValidateCategory(category)
		if err != nil {
			log.Printf("API ERROR: Invalid category provided: %v\n", err)
			writeJSON(w, http.StatusBadRequest, ErrorResponse{Error: fmt.Sprintf("Invalid category '%s': %v", category, err)})
			return
		}
		sanitizedCategories = append(sanitizedCategories, sanitized)
	}
	if err := h.storage.UpdateCategories(sanitizedCategories); err != nil {
		writeJSON(w, http.StatusInternalServerError, ErrorResponse{Error: "Failed to update categories"})
		log.Printf("API ERROR: Failed to update categories: %v\n", err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "success"})
}

func (h *Handler) RenameCategory(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPut {
		writeJSON(w, http.StatusMethodNotAllowed, ErrorResponse{Error: "Method not allowed"})
		return
	}
	var payload struct {
		OldName string `json:"oldName"`
		NewName string `json:"newName"`
	}
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		writeJSON(w, http.StatusBadRequest, ErrorResponse{Error: "Invalid request body"})
		return
	}

	// Validate the new category name
	newName, err := storage.ValidateCategory(payload.NewName)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, ErrorResponse{Error: fmt.Sprintf("Invalid new category name: %v", err)})
		return
	}

	// Get current categories
	categories, err := h.storage.GetCategories()
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, ErrorResponse{Error: "Failed to get categories"})
		log.Printf("API ERROR: Failed to get categories: %v\n", err)
		return
	}

	// Check if old category exists
	oldExists := false
	for _, cat := range categories {
		if cat == payload.OldName {
			oldExists = true
			break
		}
	}
	if !oldExists {
		writeJSON(w, http.StatusBadRequest, ErrorResponse{Error: "Old category does not exist"})
		return
	}

	// Check if new category already exists (unless it's the same as old)
	if payload.OldName != newName {
		for _, cat := range categories {
			if cat == newName {
				writeJSON(w, http.StatusBadRequest, ErrorResponse{Error: "New category name already exists"})
				return
			}
		}
	}

	// Update category in categories list
	for i, cat := range categories {
		if cat == payload.OldName {
			categories[i] = newName
			break
		}
	}

	// Update all expenses with the old category
	expenses, err := h.storage.GetAllExpenses()
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, ErrorResponse{Error: "Failed to get expenses"})
		log.Printf("API ERROR: Failed to get expenses: %v\n", err)
		return
	}

	for _, expense := range expenses {
		if expense.Category == payload.OldName {
			expense.Category = newName
			if err := h.storage.UpdateExpense(expense.ID, expense); err != nil {
				writeJSON(w, http.StatusInternalServerError, ErrorResponse{Error: "Failed to update expense"})
				log.Printf("API ERROR: Failed to update expense %s: %v\n", expense.ID, err)
				return
			}
		}
	}

	// Update all recurring expenses with the old category
	recurringExpenses, err := h.storage.GetRecurringExpenses()
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, ErrorResponse{Error: "Failed to get recurring expenses"})
		log.Printf("API ERROR: Failed to get recurring expenses: %v\n", err)
		return
	}

	for _, re := range recurringExpenses {
		if re.Category == payload.OldName {
			re.Category = newName
			if err := h.storage.UpdateRecurringExpense(re.ID, re, false); err != nil {
				writeJSON(w, http.StatusInternalServerError, ErrorResponse{Error: "Failed to update recurring expense"})
				log.Printf("API ERROR: Failed to update recurring expense %s: %v\n", re.ID, err)
				return
			}
		}
	}

	// Save updated categories
	if err := h.storage.UpdateCategories(categories); err != nil {
		writeJSON(w, http.StatusInternalServerError, ErrorResponse{Error: "Failed to update categories"})
		log.Printf("API ERROR: Failed to update categories: %v\n", err)
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{"status": "success"})
}

func (h *Handler) GetCurrency(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeJSON(w, http.StatusMethodNotAllowed, ErrorResponse{Error: "Method not allowed"})
		return
	}
	currency, err := h.storage.GetCurrency()
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, ErrorResponse{Error: "Failed to get currency"})
		log.Printf("API ERROR: Failed to get currency: %v\n", err)
		return
	}
	writeJSON(w, http.StatusOK, currency)
}

func (h *Handler) UpdateCurrency(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPut {
		writeJSON(w, http.StatusMethodNotAllowed, ErrorResponse{Error: "Method not allowed"})
		return
	}
	var currency string
	if err := json.NewDecoder(r.Body).Decode(&currency); err != nil {
		writeJSON(w, http.StatusBadRequest, ErrorResponse{Error: "Invalid request body"})
		return
	}
	if err := h.storage.UpdateCurrency(currency); err != nil {
		writeJSON(w, http.StatusInternalServerError, ErrorResponse{Error: err.Error()})
		log.Printf("API ERROR: Failed to update currency: %v\n", err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "success"})
}

func (h *Handler) GetStartDate(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeJSON(w, http.StatusMethodNotAllowed, ErrorResponse{Error: "Method not allowed"})
		return
	}
	startDate, err := h.storage.GetStartDate()
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, ErrorResponse{Error: "Failed to get start date"})
		log.Printf("API ERROR: Failed to get start date: %v\n", err)
		return
	}
	writeJSON(w, http.StatusOK, startDate)
}

func (h *Handler) UpdateStartDate(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPut {
		writeJSON(w, http.StatusMethodNotAllowed, ErrorResponse{Error: "Method not allowed"})
		return
	}
	var startDate int
	if err := json.NewDecoder(r.Body).Decode(&startDate); err != nil {
		writeJSON(w, http.StatusBadRequest, ErrorResponse{Error: "Invalid request body"})
		return
	}
	if err := h.storage.UpdateStartDate(startDate); err != nil {
		writeJSON(w, http.StatusInternalServerError, ErrorResponse{Error: err.Error()})
		log.Printf("API ERROR: Failed to update start date: %v\n", err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "success"})
}

func (h *Handler) GetLanguage(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeJSON(w, http.StatusMethodNotAllowed, ErrorResponse{Error: "Method not allowed"})
		return
	}
	language, err := h.storage.GetLanguage()
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, ErrorResponse{Error: "Failed to get language"})
		log.Printf("API ERROR: Failed to get language: %v\n", err)
		return
	}
	writeJSON(w, http.StatusOK, language)
}

func (h *Handler) UpdateLanguage(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPut {
		writeJSON(w, http.StatusMethodNotAllowed, ErrorResponse{Error: "Method not allowed"})
		return
	}
	var language string
	if err := json.NewDecoder(r.Body).Decode(&language); err != nil {
		writeJSON(w, http.StatusBadRequest, ErrorResponse{Error: "Invalid request body"})
		return
	}
	if err := h.storage.UpdateLanguage(language); err != nil {
		writeJSON(w, http.StatusInternalServerError, ErrorResponse{Error: err.Error()})
		log.Printf("API ERROR: Failed to update language: %v\n", err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "success"})
}

// ------------------------------------------------------------
// Expense Handlers
// ------------------------------------------------------------

func (h *Handler) AddExpense(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPut {
		writeJSON(w, http.StatusMethodNotAllowed, ErrorResponse{Error: "Method not allowed"})
		return
	}
	var expense storage.Expense
	if err := json.NewDecoder(r.Body).Decode(&expense); err != nil {
		writeJSON(w, http.StatusBadRequest, ErrorResponse{Error: "Invalid request body"})
		return
	}
	if err := expense.Validate(); err != nil {
		writeJSON(w, http.StatusBadRequest, ErrorResponse{Error: err.Error()})
		return
	}
	if expense.Date.IsZero() {
		expense.Date = time.Now()
	}
	if err := h.storage.AddExpense(expense); err != nil {
		writeJSON(w, http.StatusInternalServerError, ErrorResponse{Error: "Failed to save expense"})
		log.Printf("API ERROR: Failed to save expense: %v\n", err)
		return
	}
	writeJSON(w, http.StatusOK, expense)
}

func (h *Handler) GetExpenses(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeJSON(w, http.StatusMethodNotAllowed, ErrorResponse{Error: "Method not allowed"})
		return
	}
	expenses, err := h.storage.GetAllExpenses()
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, ErrorResponse{Error: "Failed to retrieve expenses"})
		log.Printf("API ERROR: Failed to retrieve expenses: %v\n", err)
		return
	}
	writeJSON(w, http.StatusOK, expenses)
}

func (h *Handler) EditExpense(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPut {
		writeJSON(w, http.StatusMethodNotAllowed, ErrorResponse{Error: "Method not allowed"})
		return
	}
	id := r.URL.Query().Get("id")
	if id == "" {
		writeJSON(w, http.StatusBadRequest, ErrorResponse{Error: "ID parameter is required"})
		return
	}
	var expense storage.Expense
	if err := json.NewDecoder(r.Body).Decode(&expense); err != nil {
		writeJSON(w, http.StatusBadRequest, ErrorResponse{Error: "Invalid request body"})
		return
	}
	if err := expense.Validate(); err != nil {
		writeJSON(w, http.StatusBadRequest, ErrorResponse{Error: err.Error()})
		return
	}
	if err := h.storage.UpdateExpense(id, expense); err != nil {
		writeJSON(w, http.StatusInternalServerError, ErrorResponse{Error: "Failed to edit expense"})
		log.Printf("API ERROR: Failed to edit expense: %v\n", err)
		return
	}
	writeJSON(w, http.StatusOK, expense)
}

func (h *Handler) DeleteExpense(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodDelete {
		writeJSON(w, http.StatusMethodNotAllowed, ErrorResponse{Error: "Method not allowed"})
		return
	}
	id := r.URL.Query().Get("id")
	if id == "" {
		writeJSON(w, http.StatusBadRequest, ErrorResponse{Error: "ID parameter is required"})
		return
	}
	if err := h.storage.RemoveExpense(id); err != nil {
		writeJSON(w, http.StatusInternalServerError, ErrorResponse{Error: "Failed to delete expense"})
		log.Printf("API ERROR: Failed to delete expense: %v\n", err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "success"})
}

func (h *Handler) DeleteMultipleExpenses(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodDelete {
		writeJSON(w, http.StatusMethodNotAllowed, ErrorResponse{Error: "Method not allowed"})
		return
	}
	var payload struct {
		IDs []string `json:"ids"`
	}
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		writeJSON(w, http.StatusBadRequest, ErrorResponse{Error: "Invalid request body"})
		return
	}
	if err := h.storage.RemoveMultipleExpenses(payload.IDs); err != nil {
		writeJSON(w, http.StatusInternalServerError, ErrorResponse{Error: "Failed to delete multiple expenses"})
		log.Printf("API ERROR: Failed to delete multiple expenses: %v\n", err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "success"})
}

// ------------------------------------------------------------
// Recurring Expense Handlers
// ------------------------------------------------------------

func (h *Handler) AddRecurringExpense(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPut {
		writeJSON(w, http.StatusMethodNotAllowed, ErrorResponse{Error: "Method not allowed"})
		return
	}
	var re storage.RecurringExpense
	if err := json.NewDecoder(r.Body).Decode(&re); err != nil {
		writeJSON(w, http.StatusBadRequest, ErrorResponse{Error: "Invalid request body"})
		return
	}
	if err := re.Validate(); err != nil {
		writeJSON(w, http.StatusBadRequest, ErrorResponse{Error: err.Error()})
		return
	}
	if err := h.storage.AddRecurringExpense(re); err != nil {
		writeJSON(w, http.StatusInternalServerError, ErrorResponse{Error: "Failed to add recurring expense"})
		log.Printf("API ERROR: Failed to add recurring expense: %v\n", err)
		return
	}
	writeJSON(w, http.StatusCreated, re)
}

func (h *Handler) GetRecurringExpenses(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeJSON(w, http.StatusMethodNotAllowed, ErrorResponse{Error: "Method not allowed"})
		return
	}
	res, err := h.storage.GetRecurringExpenses()
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, ErrorResponse{Error: "Failed to get recurring expenses"})
		log.Printf("API ERROR: Failed to get recurring expenses: %v\n", err)
		return
	}
	writeJSON(w, http.StatusOK, res)
}

func (h *Handler) UpdateRecurringExpense(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPut {
		writeJSON(w, http.StatusMethodNotAllowed, ErrorResponse{Error: "Method not allowed"})
		return
	}
	id := r.URL.Query().Get("id")
	if id == "" {
		writeJSON(w, http.StatusBadRequest, ErrorResponse{Error: "ID parameter is required"})
		return
	}
	updateAll, _ := strconv.ParseBool(r.URL.Query().Get("updateAll"))

	var re storage.RecurringExpense
	if err := json.NewDecoder(r.Body).Decode(&re); err != nil {
		writeJSON(w, http.StatusBadRequest, ErrorResponse{Error: "Invalid request body"})
		return
	}
	if err := re.Validate(); err != nil {
		writeJSON(w, http.StatusBadRequest, ErrorResponse{Error: err.Error()})
		return
	}
	if err := h.storage.UpdateRecurringExpense(id, re, updateAll); err != nil {
		writeJSON(w, http.StatusInternalServerError, ErrorResponse{Error: "Failed to update recurring expense"})
		log.Printf("API ERROR: Failed to update recurring expense: %v\n", err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "success"})
}

func (h *Handler) DeleteRecurringExpense(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodDelete {
		writeJSON(w, http.StatusMethodNotAllowed, ErrorResponse{Error: "Method not allowed"})
		return
	}
	id := r.URL.Query().Get("id")
	if id == "" {
		writeJSON(w, http.StatusBadRequest, ErrorResponse{Error: "ID parameter is required"})
		return
	}
	removeAll, _ := strconv.ParseBool(r.URL.Query().Get("removeAll"))

	if err := h.storage.RemoveRecurringExpense(id, removeAll); err != nil {
		writeJSON(w, http.StatusInternalServerError, ErrorResponse{Error: "Failed to delete recurring expense"})
		log.Printf("API ERROR: Failed to delete recurring expense: %v\n", err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "success"})
}

// ------------------------------------------------------------
// Static and UI Handlers
// ------------------------------------------------------------

func (h *Handler) ServeTableView(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeJSON(w, http.StatusMethodNotAllowed, ErrorResponse{Error: "Method not allowed"})
		return
	}
	w.Header().Set("Content-Type", "text/html")
	if err := web.ServeTemplate(w, "table.html"); err != nil {
		http.Error(w, "Failed to serve template", http.StatusInternalServerError)
	}
}

func (h *Handler) ServeSettingsPage(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeJSON(w, http.StatusMethodNotAllowed, ErrorResponse{Error: "Method not allowed"})
		return
	}
	w.Header().Set("Content-Type", "text/html")
	if err := web.ServeTemplate(w, "settings.html"); err != nil {
		http.Error(w, "Failed to serve template", http.StatusInternalServerError)
	}
}

func (h *Handler) ServeSummaryPage(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeJSON(w, http.StatusMethodNotAllowed, ErrorResponse{Error: "Method not allowed"})
		return
	}
	w.Header().Set("Content-Type", "text/html")
	if err := web.ServeTemplate(w, "summary.html"); err != nil {
		http.Error(w, "Failed to serve template", http.StatusInternalServerError)
	}
}

func (h *Handler) ServeStaticFile(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeJSON(w, http.StatusMethodNotAllowed, ErrorResponse{Error: "Method not allowed"})
		return
	}
	if err := web.ServeStatic(w, r.URL.Path); err != nil {
		http.Error(w, "Failed to serve static file", http.StatusInternalServerError)
	}
}
