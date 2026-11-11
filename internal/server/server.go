package server

import (
	"context"
	"net/http"
	"sync"

	"github.com/gokuls-codes/go-gen-ui/internal/utils"
	"github.com/gokuls-codes/go-gen-ui/internal/views/components"
	"github.com/gokuls-codes/go-gen-ui/internal/views/pages"
	"github.com/google/uuid"
	"github.com/labstack/echo/v5"
	"github.com/labstack/echo/v5/middleware"
	"google.golang.org/genai"
)

type Server struct {
	addr     string
	aiClient *genai.Client
	mu       sync.RWMutex
	threads  map[string]*genai.Chat
}

func NewServer(addr string, aiClient *genai.Client) *Server {
	return &Server{
		addr:     addr,
		aiClient: aiClient,
		threads:  make(map[string]*genai.Chat),
	}
}

func (s *Server) getOrCreateChat(ctx context.Context, threadId string) (*genai.Chat, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if chat, exists := s.threads[threadId]; exists {
		return chat, nil
	}

	chat, err := s.aiClient.Chats.Create(ctx, "gemini-2.5-flash", &genai.GenerateContentConfig{}, nil)
	if err != nil {
		return nil, err
	}
	s.threads[threadId] = chat
	return chat, nil
}

func (s *Server) Start() error {
	e := echo.New()

	e.Use(middleware.RequestLogger())
	e.Use(middleware.Recover())
	e.Static("/static", "static")

	e.GET("/", func(c *echo.Context) error {
		threadId := uuid.New().String()
		return utils.Render(c, http.StatusOK, pages.Index(threadId))
	})

	e.POST("/chat", func(c *echo.Context) error {
		userMessage := c.FormValue("message")
		threadId := c.FormValue("thread_id")

		if userMessage == "" || threadId == "" {
			return c.NoContent(http.StatusBadRequest)
		}
		chat, err := s.getOrCreateChat(c.Request().Context(), threadId)
		if err != nil {
			return utils.Render(c, http.StatusOK, components.ChatExchange(
				userMessage,
				"⚠️ Failed to initialize chat session: "+err.Error(),
			))
		}

		resp, err := chat.SendMessage(c.Request().Context(), genai.Part{Text: userMessage})
		if err != nil {
			return utils.Render(c, http.StatusOK, components.ChatExchange(
				userMessage,
				"⚠️ Error from Gemini: "+err.Error(),
			))
		}

		return utils.Render(c, http.StatusOK, components.ChatExchange(
			userMessage,
			resp.Text(),
		))
	})

	return e.Start(s.addr)
}
