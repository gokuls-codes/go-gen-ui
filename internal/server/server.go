package server

import (
	"context"
	"fmt"
	"html"
	"net/http"
	"strings"
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
	messages map[string]string
}

func NewServer(addr string, aiClient *genai.Client) *Server {
	return &Server{
		addr:     addr,
		aiClient: aiClient,
		threads:  make(map[string]*genai.Chat),
		messages: make(map[string]string),
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
		msgId := uuid.New().String()

		if userMessage == "" || threadId == "" {
			return c.NoContent(http.StatusBadRequest)
		}

		s.mu.Lock()
		s.messages[msgId] = userMessage
		s.mu.Unlock()

		return utils.Render(c, http.StatusOK, components.ChatMessageStream(
			userMessage,
			msgId,
			threadId,
		))
	})

	e.GET("/chat/stream", func(c *echo.Context) error {
		threadId := c.QueryParam("thread_id")
		msgId := c.QueryParam("msg_id")

		if threadId == "" || msgId == "" {
			return c.NoContent(http.StatusBadRequest)
		}

		s.mu.Lock()
		userMessage, ok := s.messages[msgId]
		delete(s.messages, msgId)
		s.mu.Unlock()

		if !ok {
			return c.NoContent(http.StatusNotFound)
		}

		chat, err := s.getOrCreateChat(c.Request().Context(), threadId)
		if err != nil {
			return err
		}

		w := c.Response()
		w.Header().Set("Content-Type", "text/event-stream")
		w.Header().Set("Cache-Control", "no-cache")
		w.Header().Set("Connection", "keep-alive")
		w.WriteHeader(http.StatusOK)

		rc := http.NewResponseController(w)
		rc.Flush()

		firstChunk := true
		for resp, err := range chat.SendMessageStream(c.Request().Context(), genai.Part{Text: userMessage}) {
			if err != nil {
				if firstChunk {
					fmt.Fprint(w, "event: status\ndata: \n\n")
					firstChunk = false
				}
				safeErr := html.EscapeString(err.Error())
				fmt.Fprintf(w, "event: chunk\ndata: ⚠️ Error: %s\n\n", strings.ReplaceAll(safeErr, "\n", " "))
				rc.Flush()
				break
			}

			if firstChunk {
				fmt.Fprint(w, "event: status\ndata: \n\n")
				rc.Flush()
				firstChunk = false
			}

			if text := resp.Text(); text != "" {
				safeHtml := html.EscapeString(text)
				fmt.Fprintf(w, "event: chunk\ndata: %s\n\n", strings.ReplaceAll(safeHtml, "\n", "<br>"))
				rc.Flush()
			}
		}

		fmt.Fprint(w, "event: done\ndata: complete\n\n")
		rc.Flush()

		return nil
	})

	return e.Start(s.addr)
}
