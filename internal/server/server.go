package server

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strings"
	"sync"

	"github.com/a-h/templ"
	"github.com/gokuls-codes/go-gen-ui/internal/ai"
	"github.com/gokuls-codes/go-gen-ui/internal/expenses"
	"github.com/gokuls-codes/go-gen-ui/internal/genui"
	"github.com/gokuls-codes/go-gen-ui/internal/utils"
	"github.com/gokuls-codes/go-gen-ui/internal/views/components"
	"github.com/gokuls-codes/go-gen-ui/internal/views/pages"
	"github.com/google/uuid"
	"github.com/labstack/echo/v5"
	"github.com/labstack/echo/v5/middleware"
	"google.golang.org/genai"
)

const maxToolTurns = 5

type Server struct {
	addr     string
	aiClient *ai.Client
	expenses *expenses.Store
	mu       sync.Mutex
	messages map[string]string
}

func NewServer(addr string, aiClient *ai.Client, expenseStore *expenses.Store) *Server {
	return &Server{
		addr:     addr,
		aiClient: aiClient,
		expenses: expenseStore,
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
	e.POST("/actions/submit", s.handleActionSubmit)

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

	return s.replyWithAI(c, threadId, userMessage)
}

func (s *Server) handleActionSubmit(c *echo.Context) error {
	threadId := c.FormValue("thread_id")
	if threadId == "" {
		return c.NoContent(http.StatusBadRequest)
	}

	formParams, err := c.FormValues()
	if err != nil {
		return c.NoContent(http.StatusBadRequest)
	}

	fields := make(map[string]string, len(formParams))
	for key, vals := range formParams {
		if key == "thread_id" || len(vals) == 0 {
			continue
		}
		fields[key] = vals[0]
	}

	rawFields, err := json.Marshal(fields)
	if err != nil {
		return c.NoContent(http.StatusBadRequest)
	}

	prompt := fmt.Sprintf("User submitted the UI form with data: %s", string(rawFields))
	if intent := strings.TrimSpace(c.QueryParam("intent")); intent != "" {
		prompt = fmt.Sprintf("User submitted the UI form (intent: %s) with data: %s", intent, string(rawFields))
	}

	return s.replyWithAI(c, threadId, prompt)
}

func (s *Server) replyWithAI(c *echo.Context, threadId, userMessage string) error {
	chat, err := s.aiClient.GetOrCreateChat(c.Request().Context(), threadId)
	if err != nil {
		log.Printf("chat init error: %v", err)
		return utils.Render(c, http.StatusOK, components.ChatMessage("⚠️ Error: "+err.Error(), false))
	}

	text, widgets, err := s.runChatTurn(c.Request().Context(), chat, genai.Part{Text: userMessage})
	if err != nil {
		log.Printf("gemini error: %v", err)
		return utils.Render(c, http.StatusOK, components.ChatMessage("⚠️ Error: "+err.Error(), false))
	}

	if text == "" && len(widgets) == 0 {
		text = "(No response from model)"
	}

	return utils.Render(c, http.StatusOK, components.ChatAssistantResponse(text, widgets))
}

func (s *Server) runChatTurn(ctx context.Context, chat *genai.Chat, initial genai.Part) (string, []templ.Component, error) {
	var texts []string
	var widgets []templ.Component

	parts := []genai.Part{initial}
	retriedMalformed := false

	for turn := 0; turn < maxToolTurns; turn++ {
		resp, err := chat.SendMessage(ctx, parts...)
		if err != nil {
			return "", nil, err
		}

		if len(widgets) == 0 {
			if t := strings.TrimSpace(resp.Text()); t != "" {
				texts = append(texts, t)
			}
		}

		fcs := resp.FunctionCalls()
		if len(fcs) == 0 {
			if len(resp.Candidates) > 0 {
				cand := resp.Candidates[0]
				if cand.FinishReason == genai.FinishReasonMalformedFunctionCall && !retriedMalformed {
					retriedMalformed = true
					log.Printf("retrying after MALFORMED_FUNCTION_CALL")
					continue
				}
				if len(texts) == 0 && len(widgets) == 0 {
					log.Printf("empty gemini response: finishReason=%s finishMessage=%q parts=%d", cand.FinishReason, cand.FinishMessage, candidatePartCount(cand))
					if cand.FinishReason != "" && cand.FinishReason != genai.FinishReasonStop {
						texts = append(texts, "⚠️ Gemini stopped ("+string(cand.FinishReason)+"): "+cand.FinishMessage)
					}
				}
			}
			break
		}

		funcResponses := make([]genai.Part, 0, len(fcs))
		for _, fc := range fcs {
			log.Printf("tool call: %s args=%+v", fc.Name, fc.Args)
			funcResponses = append(funcResponses, genai.Part{
				FunctionResponse: &genai.FunctionResponse{
					ID:       fc.ID,
					Name:     fc.Name,
					Response: s.executeTool(fc, &widgets),
				},
			})
		}
		parts = funcResponses
	}

	return strings.Join(texts, "\n\n"), widgets, nil
}

func (s *Server) executeTool(fc *genai.FunctionCall, widgets *[]templ.Component) map[string]any {
	switch fc.Name {
	case "render_ui":
		comp, err := buildWidget(fc.Args)
		if err != nil {
			log.Printf("render_ui build error: %v", err)
			return map[string]any{"error": err.Error()}
		}
		*widgets = append(*widgets, comp)
		return map[string]any{"output": "UI widget rendered and displayed to the user. Reply briefly with 'Done.' and do not repeat any data or markdown tables."}

	case "get_expenses":
		res := s.expenses.Query(argString(fc.Args, "category"), argString(fc.Args, "payment_mode"))
		return formatExpensesForAI(res)

	case "add_expense":
		amount, _ := fc.Args["amount"].(float64)
		added, err := s.expenses.Add(
			argString(fc.Args, "category"),
			argString(fc.Args, "payment_mode"),
			argString(fc.Args, "description"),
			amount,
		)
		if err != nil {
			return map[string]any{"error": err.Error()}
		}
		out, err := toMap(added)
		if err != nil {
			return map[string]any{"error": err.Error()}
		}
		return out

	default:
		return map[string]any{"error": "unknown tool: " + fc.Name}
	}
}

func formatExpensesForAI(res expenses.QueryResult) map[string]any {
	items := make([]map[string]any, 0, len(res.Expenses))
	for _, e := range res.Expenses {
		items = append(items, map[string]any{
			"date":        e.CreatedAt.Format("2006-01-02"),
			"description": e.Description,
			"amount":      e.Amount,
			"category":    e.Category,
			"paymentMode": e.PaymentMode,
		})
	}

	breakdown := make([]map[string]any, 0, len(res.ByCategory))
	for _, b := range res.ByCategory {
		breakdown = append(breakdown, map[string]any{
			"category": b.Category,
			"total":    b.Total,
			"count":    b.Count,
		})
	}

	catNames := make([]string, 0, len(res.Categories))
	for _, c := range res.Categories {
		catNames = append(catNames, c.Name)
	}

	pmNames := make([]string, 0, len(res.PaymentModes))
	for _, p := range res.PaymentModes {
		pmNames = append(pmNames, p.Name)
	}

	return map[string]any{
		"totalAmount":  res.TotalAmount,
		"count":        res.Count,
		"byCategory":   breakdown,
		"expenses":     items,
		"categories":   catNames,
		"paymentModes": pmNames,
	}
}

func argString(args map[string]any, key string) string {
	s, _ := args[key].(string)
	return s
}

func toMap(v any) (map[string]any, error) {
	raw, err := json.Marshal(v)
	if err != nil {
		return nil, err
	}
	var out map[string]any
	if err := json.Unmarshal(raw, &out); err != nil {
		return nil, err
	}
	return out, nil
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
