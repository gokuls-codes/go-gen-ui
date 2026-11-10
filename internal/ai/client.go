package ai

import (
	"context"

	"google.golang.org/genai"
)

func NewClient() (*genai.Client, error) {
	return genai.NewClient(context.Background(), &genai.ClientConfig{})
}
