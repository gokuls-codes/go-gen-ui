package server

import (
	"context"
	"log"
	"net/http"
	"strings"
	"sync"

	"github.com/a-h/templ"
	"github.com/gokuls-codes/go-gen-ui/internal/ai"
	"github.com/gokuls-codes/go-gen-ui/internal/genui"
	"github.com/gokuls-codes/go-gen-ui/internal/utils"
	"github.com/gokuls-codes/go-gen-ui/internal/views/components"
	"github.com/gokuls-codes/go-gen-ui/internal/views/pages"
	"github.com/google/uuid"
	"github.com/labstack/echo/v5"
	"github.com/labstack/echo/v5/middleware"
	"google.golang.org/genai"
)

const maxToolTurns = 3

type Server struct {
	addr     string
	aiClient *ai.Client
	mu       sync.Mutex
	messages map[string]string
}

func NewServer(addr string, aiClient *ai.Client) *Server {
	return &Server{
		addr:     addr,
		aiClient: aiClient,
		messages: make(map[string]string),
	}
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

		return utils.Render(c, http.StatusOK, components.ChatPendingExchange(
			userMessage,
			msgId,
			threadId,
		))
	})

	e.POST("/chat/reply", s.handleChatReply)

	return e.Start(s.addr)
}

func (s *Server) handleChatReply(c *echo.Context) error {
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

	chat, err := s.aiClient.GetOrCreateChat(c.Request().Context(), threadId)
	if err != nil {
		log.Printf("chat init error: %v", err)
		return utils.Render(c, http.StatusOK, components.ChatMessage("⚠️ Error: "+err.Error(), false))
	}

	text, widgets, err := runChatTurn(c.Request().Context(), chat, genai.Part{Text: userMessage})
	if err != nil {
		log.Printf("gemini error: %v", err)
		return utils.Render(c, http.StatusOK, components.ChatMessage("⚠️ Error: "+err.Error(), false))
	}

	if text == "" && len(widgets) == 0 {
		text = "(No response from model)"
	}

	return utils.Render(c, http.StatusOK, components.ChatAssistantResponse(text, widgets))
}

func runChatTurn(ctx context.Context, chat *genai.Chat, initial genai.Part) (string, []templ.Component, error) {
	var texts []string
	var widgets []templ.Component

	parts := []genai.Part{initial}

	for turn := 0; turn < maxToolTurns; turn++ {
		resp, err := chat.SendMessage(ctx, parts...)
		if err != nil {
			return "", nil, err
		}

		if t := strings.TrimSpace(resp.Text()); t != "" {
			texts = append(texts, t)
		}

		fcs := resp.FunctionCalls()
		if len(fcs) == 0 {
			if len(texts) == 0 && len(widgets) == 0 && len(resp.Candidates) > 0 {
				cand := resp.Candidates[0]
				log.Printf("empty gemini response: finishReason=%s finishMessage=%q parts=%d", cand.FinishReason, cand.FinishMessage, candidatePartCount(cand))
				if cand.FinishReason != "" && cand.FinishReason != genai.FinishReasonStop {
					texts = append(texts, "⚠️ Gemini stopped ("+string(cand.FinishReason)+"): "+cand.FinishMessage)
				}
			}
			break
		}

		var funcResponses []genai.Part
		for _, fc := range fcs {
			log.Printf("tool call: %s args=%+v", fc.Name, fc.Args)
			if fc.Name != "render_ui" {
				continue
			}

			result := map[string]any{"output": "UI widget rendered successfully"}
			comp, err := buildWidget(fc.Args)
			if err != nil {
				log.Printf("render_ui build error: %v", err)
				result = map[string]any{"error": err.Error()}
			} else {
				widgets = append(widgets, comp)
			}

			funcResponses = append(funcResponses, genai.Part{
				FunctionResponse: &genai.FunctionResponse{
					ID:       fc.ID,
					Name:     fc.Name,
					Response: result,
				},
			})
		}

		if len(funcResponses) == 0 {
			break
		}
		parts = funcResponses
	}

	return strings.Join(texts, "\n\n"), widgets, nil
}

func buildWidget(args map[string]any) (templ.Component, error) {
	payload, err := ai.DecodeUIPayload(args)
	if err != nil {
		return nil, err
	}
	return genui.Build(payload)
}

func candidatePartCount(cand *genai.Candidate) int {
	if cand == nil || cand.Content == nil {
		return 0
	}
	return len(cand.Content.Parts)
}
