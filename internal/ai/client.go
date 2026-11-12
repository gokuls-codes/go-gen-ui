package ai

import (
	"context"
	"encoding/json"
	"sync"

	"github.com/gokuls-codes/go-gen-ui/internal/genui"
	"google.golang.org/genai"
)

const defaultModel = "gemini-2.5-flash"

// Client manages the underlying Gemini client and multi-turn chat threads.
type Client struct {
	genai   *genai.Client
	mu      sync.RWMutex
	threads map[string]*genai.Chat
}

// NewClient initializes a new AI client with thread management.
func NewClient() (*Client, error) {
	c, err := genai.NewClient(context.Background(), &genai.ClientConfig{})
	if err != nil {
		return nil, err
	}
	return &Client{
		genai:   c,
		threads: make(map[string]*genai.Chat),
	}, nil
}

// GetOrCreateChat returns an existing chat thread or creates a new one configured with GenUI tools.
func (c *Client) GetOrCreateChat(ctx context.Context, threadID string) (*genai.Chat, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if chat, exists := c.threads[threadID]; exists {
		return chat, nil
	}

	chat, err := c.genai.Chats.Create(ctx, defaultModel, &genai.GenerateContentConfig{
		SystemInstruction: genuiSystemInstruction,
		Tools:             aiTools,
	}, nil)
	if err != nil {
		return nil, err
	}

	c.threads[threadID] = chat
	return chat, nil
}

// DecodeUIPayload converts a render_ui function call's arguments into a genui.UIPayload.
func DecodeUIPayload(args map[string]any) (genui.UIPayload, error) {
	var payload genui.UIPayload
	raw, err := json.Marshal(args)
	if err != nil {
		return payload, err
	}
	err = json.Unmarshal(raw, &payload)
	return payload, err
}
