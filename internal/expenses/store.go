package expenses

import (
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"os"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
)

type Category struct {
	ID        string    `json:"id"`
	Name      string    `json:"name"`
	Color     string    `json:"color"`
	CreatedAt time.Time `json:"createdAt"`
}

type PaymentMode struct {
	ID        string    `json:"id"`
	Name      string    `json:"name"`
	CreatedAt time.Time `json:"createdAt"`
}

type Expense struct {
	ID            string    `json:"id"`
	CategoryID    string    `json:"categoryId"`
	Amount        float64   `json:"amount"`
	Description   string    `json:"description"`
	CreatedAt     time.Time `json:"createdAt"`
	PaymentModeID *string   `json:"paymentModeId,omitempty"`
}

// ExpenseView enriches an Expense with resolved Category and PaymentMode names.
type ExpenseView struct {
	ID              string    `json:"id"`
	CategoryID      string    `json:"categoryId"`
	Category        string    `json:"category"`
	CategoryColor   string    `json:"categoryColor"`
	Amount          float64   `json:"amount"`
	Description     string    `json:"description"`
	CreatedAt       time.Time `json:"createdAt"`
	PaymentModeID   string    `json:"paymentModeId,omitempty"`
	PaymentMode     string    `json:"paymentMode,omitempty"`
}

type CategorySummary struct {
	CategoryID string  `json:"categoryId"`
	Category   string  `json:"category"`
	Color      string  `json:"color"`
	Total      float64 `json:"total"`
	Count      int     `json:"count"`
}

type QueryResult struct {
	Expenses     []ExpenseView     `json:"expenses"`
	TotalAmount  float64           `json:"totalAmount"`
	Count        int               `json:"count"`
	ByCategory   []CategorySummary `json:"byCategory"`
	Categories   []Category        `json:"categories"`
	PaymentModes []PaymentMode     `json:"paymentModes"`
}

type storeData struct {
	Categories   []Category    `json:"categories"`
	PaymentModes []PaymentMode `json:"paymentModes"`
	Expenses     []Expense     `json:"expenses"`
}

// Store holds expense data loaded from a JSON file and persists updates back to disk.
type Store struct {
	mu   sync.RWMutex
	path string
	data storeData
}

// NewStore loads expense data from a JSON file at path.
func NewStore(path string) (*Store, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read expenses file: %w", err)
	}

	var data storeData
	if err := json.Unmarshal(raw, &data); err != nil {
		return nil, fmt.Errorf("parse expenses file: %w", err)
	}

	return &Store{
		path: path,
		data: data,
	}, nil
}

// Query filters expenses by optional category and payment mode (by ID or partial name match).
func (s *Store) Query(categoryFilter, paymentModeFilter string) QueryResult {
	s.mu.RLock()
	defer s.mu.RUnlock()

	catMap := s.categoryMapLocked()
	pmMap := s.paymentModeMapLocked()

	catFilter := strings.TrimSpace(strings.ToLower(categoryFilter))
	pmFilter := strings.TrimSpace(strings.ToLower(paymentModeFilter))

	var matched []ExpenseView
	var total float64
	summaryMap := make(map[string]*CategorySummary)

	for _, exp := range s.data.Expenses {
		cat := catMap[exp.CategoryID]
		var pm PaymentMode
		if exp.PaymentModeID != nil {
			pm = pmMap[*exp.PaymentModeID]
		}

		if catFilter != "" && !matchesTerm(catFilter, cat.ID, cat.Name) {
			continue
		}
		if pmFilter != "" && !matchesTerm(pmFilter, pm.ID, pm.Name) {
			continue
		}

		view := ExpenseView{
			ID:            exp.ID,
			CategoryID:    exp.CategoryID,
			Category:      cat.Name,
			CategoryColor: cat.Color,
			Amount:        round2(exp.Amount),
			Description:   exp.Description,
			CreatedAt:     exp.CreatedAt,
			PaymentModeID: pm.ID,
			PaymentMode:   pm.Name,
		}
		matched = append(matched, view)
		total += exp.Amount

		sum, exists := summaryMap[exp.CategoryID]
		if !exists {
			sum = &CategorySummary{
				CategoryID: exp.CategoryID,
				Category:   cat.Name,
				Color:      cat.Color,
			}
			summaryMap[exp.CategoryID] = sum
		}
		sum.Total = round2(sum.Total + exp.Amount)
		sum.Count++
	}

	// Sort newest first so recent expenses appear at the top.
	sort.Slice(matched, func(i, j int) bool {
		return matched[i].CreatedAt.After(matched[j].CreatedAt)
	})

	byCategory := make([]CategorySummary, 0, len(summaryMap))
	for _, sum := range summaryMap {
		byCategory = append(byCategory, *sum)
	}
	sort.Slice(byCategory, func(i, j int) bool {
		return byCategory[i].Total > byCategory[j].Total
	})

	return QueryResult{
		Expenses:     matched,
		TotalAmount:  round2(total),
		Count:        len(matched),
		ByCategory:   byCategory,
		Categories:   append([]Category(nil), s.data.Categories...),
		PaymentModes: append([]PaymentMode(nil), s.data.PaymentModes...),
	}
}

// Add creates and persists a new expense, resolving category and payment mode by ID or name.
func (s *Store) Add(categoryInput, paymentModeInput, description string, amount float64) (ExpenseView, error) {
	if amount <= 0 {
		return ExpenseView{}, errors.New("amount must be greater than zero")
	}
	description = strings.TrimSpace(description)
	if description == "" {
		return ExpenseView{}, errors.New("description is required")
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	cat, ok := s.resolveCategoryLocked(categoryInput)
	if !ok {
		return ExpenseView{}, fmt.Errorf("unknown category %q", categoryInput)
	}

	var pmID *string
	var pmName string
	if strings.TrimSpace(paymentModeInput) != "" {
		pm, ok := s.resolvePaymentModeLocked(paymentModeInput)
		if !ok {
			return ExpenseView{}, fmt.Errorf("unknown payment mode %q", paymentModeInput)
		}
		idCopy := pm.ID
		pmID = &idCopy
		pmName = pm.Name
	}

	exp := Expense{
		ID:            "exp_" + uuid.NewString()[:8],
		CategoryID:    cat.ID,
		Amount:        round2(amount),
		Description:   description,
		CreatedAt:     time.Now().UTC(),
		PaymentModeID: pmID,
	}

	s.data.Expenses = append(s.data.Expenses, exp)
	if err := s.saveLocked(); err != nil {
		s.data.Expenses = s.data.Expenses[:len(s.data.Expenses)-1]
		return ExpenseView{}, err
	}

	var resolvedPMID string
	if pmID != nil {
		resolvedPMID = *pmID
	}

	return ExpenseView{
		ID:            exp.ID,
		CategoryID:    cat.ID,
		Category:      cat.Name,
		CategoryColor: cat.Color,
		Amount:        exp.Amount,
		Description:   exp.Description,
		CreatedAt:     exp.CreatedAt,
		PaymentModeID: resolvedPMID,
		PaymentMode:   pmName,
	}, nil
}

func (s *Store) saveLocked() error {
	raw, err := json.MarshalIndent(s.data, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal expenses: %w", err)
	}
	return os.WriteFile(s.path, append(raw, '\n'), 0644)
}

func (s *Store) categoryMapLocked() map[string]Category {
	m := make(map[string]Category, len(s.data.Categories))
	for _, c := range s.data.Categories {
		m[c.ID] = c
	}
	return m
}

func (s *Store) paymentModeMapLocked() map[string]PaymentMode {
	m := make(map[string]PaymentMode, len(s.data.PaymentModes))
	for _, p := range s.data.PaymentModes {
		m[p.ID] = p
	}
	return m
}

func (s *Store) resolveCategoryLocked(input string) (Category, bool) {
	term := strings.TrimSpace(strings.ToLower(input))
	if term == "" && len(s.data.Categories) > 0 {
		return s.data.Categories[0], true
	}
	for _, c := range s.data.Categories {
		if strings.ToLower(c.ID) == term || strings.ToLower(c.Name) == term {
			return c, true
		}
	}
	for _, c := range s.data.Categories {
		if strings.Contains(strings.ToLower(c.Name), term) {
			return c, true
		}
	}
	return Category{}, false
}

func (s *Store) resolvePaymentModeLocked(input string) (PaymentMode, bool) {
	term := strings.TrimSpace(strings.ToLower(input))
	for _, p := range s.data.PaymentModes {
		if strings.ToLower(p.ID) == term || strings.ToLower(p.Name) == term {
			return p, true
		}
	}
	for _, p := range s.data.PaymentModes {
		if strings.Contains(strings.ToLower(p.Name), term) {
			return p, true
		}
	}
	return PaymentMode{}, false
}

func matchesTerm(filter, id, name string) bool {
	return strings.ToLower(id) == filter || strings.Contains(strings.ToLower(name), filter)
}

func round2(v float64) float64 {
	return math.Round(v*100) / 100
}

