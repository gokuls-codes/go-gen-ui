package genui

import (
	"context"
	"strings"
	"testing"

	"github.com/gokuls-codes/go-gen-ui/internal/utils"
)

func TestBuild_FlatTreeRendering(t *testing.T) {
	payload := UIPayload{
		RootID: "card1",
		Components: []UINode{
			{
				ID:       "card1",
				Type:     "card",
				Props:    map[string]any{"title": "Feedback", "description": "Share your thoughts"},
				Children: []string{"form1"},
			},
			{
				ID:       "form1",
				Type:     "form",
				Props:    map[string]any{"action": "/actions/submit", "gap": "sm"},
				Children: []string{"input1", "btn1"},
			},
			{
				ID:    "input1",
				Type:  "input",
				Props: map[string]any{"name": "note", "label": "Note", "placeholder": "Type here...", "required": true},
			},
			{
				ID:    "btn1",
				Type:  "button",
				Props: map[string]any{"label": "Send", "type": "submit", "variant": "primary"},
			},
		},
	}

	comp, err := Build(payload)
	if err != nil {
		t.Fatalf("Build failed: %v", err)
	}

	html, err := utils.RenderToString(context.Background(), comp)
	if err != nil {
		t.Fatalf("RenderToString failed: %v", err)
	}

	checks := []string{
		"Feedback",
		"Share your thoughts",
		`hx-post="/actions/submit"`,
		`hx-include="#thread-id"`,
		`hx-target="closest form"`,
		`hx-swap="outerHTML"`,
		`name="note"`,
		`placeholder="Type here..."`,
		"Send",
		`type="submit"`,
	}
	for _, want := range checks {
		if !strings.Contains(html, want) {
			t.Errorf("expected HTML to contain %q, got:\n%s", want, html)
		}
	}
}

func TestBuild_SanitizesUnsafeProps(t *testing.T) {
	payload := UIPayload{
		RootID: "btn1",
		Components: []UINode{
			{
				ID:   "btn1",
				Type: "button",
				Props: map[string]any{
					"label":  "Click",
					"action": "https://evil.example/steal",
					"target": "body",
					"class":  "injected-evil-class",
				},
			},
		},
	}

	comp, err := Build(payload)
	if err != nil {
		t.Fatalf("Build failed: %v", err)
	}

	html, err := utils.RenderToString(context.Background(), comp)
	if err != nil {
		t.Fatalf("RenderToString failed: %v", err)
	}

	for _, forbidden := range []string{"evil.example", `hx-target="body"`, "injected-evil-class"} {
		if strings.Contains(html, forbidden) {
			t.Errorf("expected HTML NOT to contain %q, got:\n%s", forbidden, html)
		}
	}
}

func TestBuild_CycleProtection(t *testing.T) {
	payload := UIPayload{
		RootID: "a",
		Components: []UINode{
			{ID: "a", Type: "card", Children: []string{"b"}},
			{ID: "b", Type: "flex", Children: []string{"a"}},
		},
	}

	comp, err := Build(payload)
	if err != nil {
		t.Fatalf("Build failed on cycle: %v", err)
	}

	if _, err := utils.RenderToString(context.Background(), comp); err != nil {
		t.Fatalf("RenderToString failed on cycle: %v", err)
	}
}

func TestBuild_TableRendering(t *testing.T) {
	payload := UIPayload{
		RootID: "tbl1",
		Components: []UINode{
			{
				ID:   "tbl1",
				Type: "table",
				Props: map[string]any{
					"caption": "Recent Expenses",
					"headers": []any{"Item", "Amount"},
					"rows": []any{
						map[string]any{"cells": []any{"Coffee", "$6.75"}},
						map[string]any{"cells": []any{"Cab", "$14.00"}},
					},
				},
			},
		},
	}

	comp, err := Build(payload)
	if err != nil {
		t.Fatalf("Build failed: %v", err)
	}

	html, err := utils.RenderToString(context.Background(), comp)
	if err != nil {
		t.Fatalf("RenderToString failed: %v", err)
	}

	for _, want := range []string{"Recent Expenses", "<thead", "<th", "Item", "Coffee", "$6.75", "Cab", "$14.00"} {
		if !strings.Contains(html, want) {
			t.Errorf("expected HTML to contain %q, got:\n%s", want, html)
		}
	}
}

func TestBuild_ChartRendering(t *testing.T) {
	payload := UIPayload{
		RootID: "chart1",
		Components: []UINode{
			{
				ID:   "chart1",
				Type: "chart",
				Props: map[string]any{
					"title":      "Spending by Category",
					"chart_type": "doughnut",
					"items": []any{
						map[string]any{"label": "Travel", "value": float64(256), "display": "$256.00"},
						map[string]any{"label": "Groceries", "value": float64(128), "display": "$128.00"},
					},
				},
			},
		},
	}

	comp, err := Build(payload)
	if err != nil {
		t.Fatalf("Build failed: %v", err)
	}

	html, err := utils.RenderToString(context.Background(), comp)
	if err != nil {
		t.Fatalf("RenderToString failed: %v", err)
	}

	for _, want := range []string{"Spending by Category", "<canvas", "data-chart=", "doughnut", "Travel", "$256.00", "Groceries", "$128.00"} {
		if !strings.Contains(html, want) {
			t.Errorf("expected HTML to contain %q, got:\n%s", want, html)
		}
	}
}

func TestBuild_SelectRendering(t *testing.T) {
	payload := UIPayload{
		RootID: "sel1",
		Components: []UINode{
			{
				ID:   "sel1",
				Type: "select",
				Props: map[string]any{
					"name":     "category",
					"label":    "Category",
					"options":  []any{"Food & Dining", "Travel", "Groceries"},
					"value":    "food & dining",
					"required": true,
				},
			},
		},
	}

	comp, err := Build(payload)
	if err != nil {
		t.Fatalf("Build failed: %v", err)
	}

	html, err := utils.RenderToString(context.Background(), comp)
	if err != nil {
		t.Fatalf("RenderToString failed: %v", err)
	}

	for _, want := range []string{`<select`, `name="category"`, `required`, `<option value="Food &amp; Dining" selected`, `<option value="Travel"`} {
		if !strings.Contains(html, want) {
			t.Errorf("expected HTML to contain %q, got:\n%s", want, html)
		}
	}
}
