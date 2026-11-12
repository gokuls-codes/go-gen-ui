package expenses

import (
	"os"
	"path/filepath"
	"testing"
)

func setupTestStore(t *testing.T) *Store {
	t.Helper()
	seed := `{
  "categories": [
    {"id": "cat_food", "name": "Food & Dining", "color": "#f97316", "createdAt": "2026-11-01T09:00:00Z"},
    {"id": "cat_travel", "name": "Travel", "color": "#3b82f6", "createdAt": "2026-11-01T09:00:00Z"}
  ],
  "paymentModes": [
    {"id": "pm_upi", "name": "UPI", "createdAt": "2026-11-01T09:00:00Z"},
    {"id": "pm_cash", "name": "Cash", "createdAt": "2026-11-01T09:00:00Z"}
  ],
  "expenses": [
    {"id": "exp_1", "categoryId": "cat_food", "amount": 12.50, "description": "Coffee", "createdAt": "2026-11-02T10:00:00Z", "paymentModeId": "pm_upi"},
    {"id": "exp_2", "categoryId": "cat_travel", "amount": 30.00, "description": "Cab", "createdAt": "2026-11-03T10:00:00Z", "paymentModeId": "pm_cash"}
  ]
}`
	path := filepath.Join(t.TempDir(), "expenses.json")
	if err := os.WriteFile(path, []byte(seed), 0644); err != nil {
		t.Fatal(err)
	}
	store, err := NewStore(path)
	if err != nil {
		t.Fatal(err)
	}
	return store
}

func TestStore_QueryAndFilter(t *testing.T) {
	store := setupTestStore(t)

	all := store.Query("", "")
	if all.Count != 2 || all.TotalAmount != 42.50 {
		t.Fatalf("expected 2 expenses totaling 42.50, got %+v", all)
	}

	food := store.Query("food", "")
	if food.Count != 1 || food.TotalAmount != 12.50 || food.Expenses[0].PaymentMode != "UPI" {
		t.Fatalf("expected 1 food UPI expense, got %+v", food)
	}
}

func TestStore_AddAndPersist(t *testing.T) {
	store := setupTestStore(t)

	added, err := store.Add("Food & Dining", "UPI", "Dinner", 24.75)
	if err != nil {
		t.Fatalf("Add failed: %v", err)
	}
	if added.CategoryID != "cat_food" || added.PaymentModeID != "pm_upi" || added.Amount != 24.75 {
		t.Fatalf("unexpected added view: %+v", added)
	}

	// Reload from disk to confirm persistence.
	reloaded, err := NewStore(store.path)
	if err != nil {
		t.Fatal(err)
	}
	res := reloaded.Query("food", "")
	if res.Count != 2 || res.TotalAmount != 37.25 {
		t.Fatalf("expected 2 persisted food expenses totaling 37.25, got %+v", res)
	}
}

