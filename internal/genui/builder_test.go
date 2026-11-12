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
