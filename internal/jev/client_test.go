package jev

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"testing"
)

type roundTripperFunc func(*http.Request) (*http.Response, error)

func (f roundTripperFunc) RoundTrip(r *http.Request) (*http.Response, error) {
	return f(r)
}

func TestEvaluate_Success(t *testing.T) {
	t.Setenv("JEV_API_KEY", "test-api-key")

	transport := roundTripperFunc(func(r *http.Request) (*http.Response, error) {
		if r.Method != http.MethodPost {
			t.Errorf("expected POST, got %s", r.Method)
		}
		if auth := r.Header.Get("Authorization"); auth != "Bearer test-api-key" {
			t.Errorf("expected Bearer test-api-key, got %s", auth)
		}
		if ct := r.Header.Get("Content-Type"); ct != "application/json" {
			t.Errorf("expected application/json, got %s", ct)
		}

		var req Request
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			t.Fatalf("failed to decode request: %v", err)
		}

		if req.Model != "jev-latest" {
			t.Errorf("expected model jev-latest, got %s", req.Model)
		}
		if len(req.Questions) != 3 {
			t.Errorf("expected 3 questions, got %d", len(req.Questions))
		}

		noulVal := 0.98
		scoreVal := 0.75
		respObj := Response{
			Model: "jev-latest",
			Answers: map[string]Answer{
				"department": {
					Type:       QuestionTypeChoice,
					Choice:     "technical",
					Confidence: 0.95,
					Probabilities: map[string]float64{
						"billing":   0.02,
						"technical": 0.95,
						"sales":     0.03,
					},
				},
				"is_urgent": {
					Type: QuestionTypeNoul,
					Noul: &noulVal,
				},
				"frustration": {
					Type:       QuestionTypeScore,
					Score:      &scoreVal,
					Confidence: 0.88,
				},
			},
			Usage: Usage{
				InputTokens:  150,
				OutputTokens: 25,
			},
		}

		bodyBytes, _ := json.Marshal(respObj)
		return &http.Response{
			StatusCode: http.StatusOK,
			Header:     http.Header{"Content-Type": []string{"application/json"}},
			Body:       io.NopCloser(bytes.NewReader(bodyBytes)),
		}, nil
	})

	httpClient := &http.Client{Transport: transport}
	client, err := NewClient(WithHTTPClient(httpClient))
	if err != nil {
		t.Fatalf("NewClient failed: %v", err)
	}

	questions := map[string]Question{
		"department": NewChoiceQuestion("Which team should handle this", map[string]string{
			"billing":   "Payment or subscription issues",
			"technical": "Bugs or integration problems",
			"sales":     "Pricing or account questions",
		}),
		"frustration": NewScoreQuestion("How frustrated the customer appears", []string{
			"Calm, just stating facts",
			"Frustrated but civil",
			"Very angry, strong language",
		}),
		"is_urgent": NewNoulQuestion("The message conveys urgency or time-sensitivity"),
	}

	state := "Our API integration started returning 500 errors on every request about 20 minutes ago."
	res, err := client.Evaluate(context.Background(), state, questions)
	if err != nil {
		t.Fatalf("Evaluate failed: %v", err)
	}

	deptAns, ok := res.Answers["department"]
	if !ok || deptAns.Choice != "technical" || deptAns.Confidence != 0.95 {
		t.Errorf("unexpected department answer: %+v", deptAns)
	}

	urgentAns, ok := res.Answers["is_urgent"]
	if !ok || urgentAns.Noul == nil || *urgentAns.Noul != 0.98 {
		t.Errorf("unexpected is_urgent answer: %+v", urgentAns)
	}

	frustAns, ok := res.Answers["frustration"]
	if !ok || frustAns.Score == nil || *frustAns.Score != 0.75 {
		t.Errorf("unexpected frustration answer: %+v", frustAns)
	}
}

func TestConvenienceMethods(t *testing.T) {
	t.Setenv("JEV_API_KEY", "test-key")

	transport := roundTripperFunc(func(r *http.Request) (*http.Response, error) {
		var req Request
		_ = json.NewDecoder(r.Body).Decode(&req)

		q := req.Questions["q"]
		resp := Response{Model: "jev-latest", Answers: map[string]Answer{}}

		switch q.Type {
		case QuestionTypeNoul:
			val := 0.87
			resp.Answers["q"] = Answer{Type: QuestionTypeNoul, Noul: &val}
		case QuestionTypeChoice:
			resp.Answers["q"] = Answer{
				Type:       QuestionTypeChoice,
				Choice:     "Food & Dining",
				Confidence: 0.92,
				Probabilities: map[string]float64{
					"Food & Dining": 0.92,
					"Travel":        0.08,
				},
			}
		case QuestionTypeScore:
			val := 0.65
			resp.Answers["q"] = Answer{
				Type:       QuestionTypeScore,
				Score:      &val,
				Confidence: 0.80,
			}
		}

		bodyBytes, _ := json.Marshal(resp)
		return &http.Response{
			StatusCode: http.StatusOK,
			Header:     http.Header{"Content-Type": []string{"application/json"}},
			Body:       io.NopCloser(bytes.NewReader(bodyBytes)),
		}, nil
	})

	httpClient := &http.Client{Transport: transport}
	client, err := NewClient(WithHTTPClient(httpClient))
	if err != nil {
		t.Fatalf("NewClient failed: %v", err)
	}
	ctx := context.Background()

	// AskNoul
	prob, err := client.AskNoul(ctx, "I spent money on coffee", "Is this an expense?")
	if err != nil || prob != 0.87 {
		t.Errorf("AskNoul failed: got prob=%f, err=%v", prob, err)
	}

	// AskChoice
	choice, conf, probs, err := client.AskChoice(ctx, "Coffee at Starbucks", "Select category", map[string]string{
		"Food & Dining": "Restaurants and cafes",
		"Travel":        "Transit and flights",
	})
	if err != nil || choice != "Food & Dining" || conf != 0.92 || len(probs) != 2 {
		t.Errorf("AskChoice failed: got choice=%s, conf=%f, err=%v", choice, conf, err)
	}

	// AskScore
	score, scoreConf, err := client.AskScore(ctx, "Everything crashed", "Severity level", []string{"low", "medium", "critical"})
	if err != nil || score != 0.65 || scoreConf != 0.80 {
		t.Errorf("AskScore failed: got score=%f, conf=%f, err=%v", score, scoreConf, err)
	}
}

func TestEvaluate_ErrorHandling(t *testing.T) {
	t.Setenv("JEV_API_KEY", "")
	_, err := NewClient()
	if err == nil {
		t.Errorf("expected error for empty JEV_API_KEY in NewClient, got nil")
	}

	t.Setenv("JEV_API_KEY", "bad-key")
	transport := roundTripperFunc(func(r *http.Request) (*http.Response, error) {
		return &http.Response{
			StatusCode: http.StatusUnauthorized,
			Body:       io.NopCloser(bytes.NewBufferString(`{"error": "invalid api key"}`)),
		}, nil
	})

	clientWithKey, err := NewClient(WithHTTPClient(&http.Client{Transport: transport}))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	_, err = clientWithKey.Evaluate(context.Background(), nil, map[string]Question{
		"q": NewNoulQuestion("instructions"),
	})
	if err == nil {
		t.Errorf("expected error for nil state, got nil")
	}

	_, err = clientWithKey.Evaluate(context.Background(), "state", map[string]Question{
		"q": NewNoulQuestion("instructions"),
	})
	if err == nil {
		t.Errorf("expected error for HTTP 401, got nil")
	}
}

func TestEvaluate_StructuredState(t *testing.T) {
	t.Setenv("JEV_API_KEY", "test-key")

	var receivedState any
	transport := roundTripperFunc(func(r *http.Request) (*http.Response, error) {
		var req Request
		_ = json.NewDecoder(r.Body).Decode(&req)
		receivedState = req.State

		noulVal := 0.99
		respObj := Response{
			Model: "jev-latest",
			Answers: map[string]Answer{
				"is_food": {Type: QuestionTypeNoul, Noul: &noulVal},
			},
		}
		bodyBytes, _ := json.Marshal(respObj)
		return &http.Response{
			StatusCode: http.StatusOK,
			Header:     http.Header{"Content-Type": []string{"application/json"}},
			Body:       io.NopCloser(bytes.NewReader(bodyBytes)),
		}, nil
	})

	client, err := NewClient(WithHTTPClient(&http.Client{Transport: transport}))
	if err != nil {
		t.Fatalf("NewClient failed: %v", err)
	}

	// Pass map / struct as state
	stateMap := map[string]any{
		"description": "Lunch at Sweetgreen",
		"amount":      18.50,
		"location":    "NYC",
	}

	res, err := client.Evaluate(context.Background(), stateMap, map[string]Question{
		"is_food": NewNoulQuestion("Is this transaction food-related?"),
	})
	if err != nil {
		t.Fatalf("failed: %v", err)
	}

	if res.Answers["is_food"].Noul == nil || *res.Answers["is_food"].Noul != 0.99 {
		t.Errorf("unexpected answer: %+v", res.Answers)
	}

	stateRecv, ok := receivedState.(map[string]any)
	if !ok || stateRecv["description"] != "Lunch at Sweetgreen" {
		t.Errorf("expected state map to be preserved, got: %+v", receivedState)
	}
}
