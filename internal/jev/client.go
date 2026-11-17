package jev

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"time"
)

const (
	// DefaultBaseURL is the official TypeSafe System One API endpoint.
	DefaultBaseURL = "https://api.typesafe.ai/v1/systemone"

	// DefaultModel is the default Jev System One model.
	DefaultModel = "jev-latest"
)

// QuestionType identifies the primitive type of question evaluated by Jev.
type QuestionType string

const (
	// QuestionTypeNoul is a boolean question returning a probability between 0 and 1.
	QuestionTypeNoul QuestionType = "noul"

	// QuestionTypeChoice selects a single category from an explicit criteria map.
	QuestionTypeChoice QuestionType = "choice"

	// QuestionTypeScore rates an input on an ordered scale.
	QuestionTypeScore QuestionType = "score"
)

// Question represents a typed question evaluated against a state.
type Question struct {
	Type         QuestionType `json:"type"`
	Instructions string       `json:"instructions"`
	// Criteria is map[string]string for choice, or []string for score.
	Criteria any `json:"criteria,omitempty"`
}

// NewNoulQuestion creates a yes/no probability question.
func NewNoulQuestion(instructions string) Question {
	return Question{
		Type:         QuestionTypeNoul,
		Instructions: instructions,
	}
}

// NewChoiceQuestion creates a classification question with candidate criteria.
// criteria maps each choice key (e.g. "billing") to its description.
func NewChoiceQuestion(instructions string, criteria map[string]string) Question {
	return Question{
		Type:         QuestionTypeChoice,
		Instructions: instructions,
		Criteria:     criteria,
	}
}

// NewScoreQuestion creates an ordered scale evaluation question.
// criteria lists scale levels from lowest to highest.
func NewScoreQuestion(instructions string, criteria []string) Question {
	return Question{
		Type:         QuestionTypeScore,
		Instructions: instructions,
		Criteria:     criteria,
	}
}

// Request represents the payload sent to the System One API.
// State can be a string, a JSON object (map/struct), or a list/array of items.
type Request struct {
	State     any                 `json:"state"`
	Model     string              `json:"model"`
	Questions map[string]Question `json:"questions"`
	Assets    map[string]string   `json:"assets,omitempty"`
}

// Answer represents the evaluated outcome for a single question.
type Answer struct {
	Type          QuestionType       `json:"type"`
	Noul          *float64           `json:"noul,omitempty"`
	Choice        string             `json:"choice,omitempty"`
	Score         *float64           `json:"score,omitempty"`
	Confidence    float64            `json:"confidence,omitempty"`
	Probabilities map[string]float64 `json:"probabilities,omitempty"`
}

// Usage holds token consumption statistics.
type Usage struct {
	InputTokens  int `json:"input_tokens,omitempty"`
	OutputTokens int `json:"output_tokens,omitempty"`
}

// Response represents the complete API response from Jev.
type Response struct {
	Model   string            `json:"model"`
	Answers map[string]Answer `json:"answers"`
	Usage   Usage             `json:"usage,omitempty"`
}

// Client communicates with TypeSafe AI's Jev API.
type Client struct {
	apiKey     string
	baseURL    string
	model      string
	httpClient *http.Client
}

// Option configures a Client instance.
type Option func(*Client)

// WithBaseURL overrides the API endpoint URL (useful for testing or proxying).
func WithBaseURL(baseURL string) Option {
	return func(c *Client) {
		c.baseURL = baseURL
	}
}

// WithModel sets the model identifier to use (e.g. "jev-latest", "typesafe/jev-1.13").
func WithModel(model string) Option {
	return func(c *Client) {
		c.model = model
	}
}

// WithHTTPClient overrides the underlying net/http client.
func WithHTTPClient(client *http.Client) Option {
	return func(c *Client) {
		c.httpClient = client
	}
}

// NewClient initializes a Jev client.
// If apiKey is empty, it checks JEV_API_KEY environment variable.
// Returns an error if no API key is provided or found in the environment.
func NewClient(opts ...Option) (*Client, error) {
	apiKey := os.Getenv("JEV_API_KEY")
	if apiKey == "" {
		return nil, fmt.Errorf("jev: JEV_API_KEY environment variable is required")
	}

	c := &Client{
		apiKey:  apiKey,
		baseURL: DefaultBaseURL,
		model:   DefaultModel,
		httpClient: &http.Client{
			Timeout: 15 * time.Second,
		},
	}

	for _, opt := range opts {
		opt(c)
	}

	return c, nil
}

// Evaluate sends a state (string, map, struct, or slice) and a set of typed questions to Jev in parallel.
func (c *Client) Evaluate(ctx context.Context, state any, questions map[string]Question) (*Response, error) {
	if c.apiKey == "" {
		return nil, fmt.Errorf("jev: API key is not configured (set JEV_API_KEY)")
	}
	if state == nil {
		return nil, fmt.Errorf("jev: state cannot be nil")
	}
	if len(questions) == 0 {
		return nil, fmt.Errorf("jev: at least one question must be provided")
	}

	reqBody := Request{
		State:     state,
		Model:     c.model,
		Questions: questions,
	}

	payloadBytes, err := json.Marshal(reqBody)
	if err != nil {
		return nil, fmt.Errorf("jev: failed to marshal request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL, bytes.NewReader(payloadBytes))
	if err != nil {
		return nil, fmt.Errorf("jev: failed to create request: %w", err)
	}

	req.Header.Set("Authorization", "Bearer "+c.apiKey)
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("jev: http request failed: %w", err)
	}
	defer resp.Body.Close()

	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("jev: failed to read response body: %w", err)
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("jev api error (status %d): %s", resp.StatusCode, string(bodyBytes))
	}

	var res Response
	if err := json.Unmarshal(bodyBytes, &res); err != nil {
		return nil, fmt.Errorf("jev: failed to parse response JSON: %w (body: %s)", err, string(bodyBytes))
	}

	return &res, nil
}

// AskNoul is a convenience method for evaluating a single yes/no probability question.
func (c *Client) AskNoul(ctx context.Context, state any, instructions string) (float64, error) {
	res, err := c.Evaluate(ctx, state, map[string]Question{
		"q": NewNoulQuestion(instructions),
	})
	if err != nil {
		return 0, err
	}

	ans, ok := res.Answers["q"]
	if !ok || ans.Noul == nil {
		return 0, fmt.Errorf("jev: response did not contain noul answer")
	}

	return *ans.Noul, nil
}

// AskChoice is a convenience method for single classification against criteria.
func (c *Client) AskChoice(ctx context.Context, state any, instructions string, criteria map[string]string) (string, float64, map[string]float64, error) {
	res, err := c.Evaluate(ctx, state, map[string]Question{
		"q": NewChoiceQuestion(instructions, criteria),
	})
	if err != nil {
		return "", 0, nil, err
	}

	ans, ok := res.Answers["q"]
	if !ok {
		return "", 0, nil, fmt.Errorf("jev: response did not contain choice answer")
	}

	return ans.Choice, ans.Confidence, ans.Probabilities, nil
}

// AskScore is a convenience method for evaluating an ordered scale question.
func (c *Client) AskScore(ctx context.Context, state any, instructions string, criteria []string) (float64, float64, error) {
	res, err := c.Evaluate(ctx, state, map[string]Question{
		"q": NewScoreQuestion(instructions, criteria),
	})
	if err != nil {
		return 0, 0, err
	}

	ans, ok := res.Answers["q"]
	if !ok || ans.Score == nil {
		return 0, 0, fmt.Errorf("jev: response did not contain score answer")
	}

	return *ans.Score, ans.Confidence, nil
}
