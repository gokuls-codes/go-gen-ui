package server

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"regexp"
	"sort"
	"strings"
	"sync"

	"github.com/a-h/templ"
	"github.com/gokuls-codes/go-gen-ui/internal/ai"
	"github.com/gokuls-codes/go-gen-ui/internal/expenses"
	"github.com/gokuls-codes/go-gen-ui/internal/genui"
	"github.com/gokuls-codes/go-gen-ui/internal/jev"
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
	addr      string
	aiClient  *ai.Client
	jevClient *jev.Client
	expenses  *expenses.Store
	mu        sync.Mutex
	messages  map[string]string
}

func NewServer(addr string, aiClient *ai.Client, jevClient *jev.Client, expenseStore *expenses.Store) *Server {
	return &Server{
		addr:      addr,
		aiClient:  aiClient,
		jevClient: jevClient,
		expenses:  expenseStore,
		messages:  make(map[string]string),
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
	e.POST("/jev/evaluate", s.handleJevEvaluate)
	e.POST("/jev/playground", s.handleJevPlayground)

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

	trimmed := strings.TrimSpace(userMessage)
	if strings.HasPrefix(strings.ToLower(trimmed), "@jev") {
		initialState := strings.TrimSpace(trimmed[4:])
		if initialState == "" {
			initialState = "Our API started returning 500 errors on every request about 20 minutes ago, and we can't process any customer orders."
		}
		return utils.Render(c, http.StatusOK, components.JevPlayground(initialState, threadId))
	}

	return s.replyWithAI(c, threadId, userMessage)
}

func (s *Server) handleJevPlayground(c *echo.Context) error {
	state := c.FormValue("state")
	threadId := c.FormValue("thread_id")
	if state == "" {
		state = "Our API started returning 500 errors on every request about 20 minutes ago, and we can't process any customer orders."
	}
	return utils.Render(c, http.StatusOK, components.JevPlayground(state, threadId))
}

func (s *Server) handleJevEvaluate(c *echo.Context) error {
	if s.jevClient == nil {
		return utils.Render(c, http.StatusOK, components.ChatMessage("⚠️ Jev client is not initialized.", false))
	}

	stateStr := strings.TrimSpace(c.FormValue("state"))
	qType := strings.TrimSpace(c.FormValue("question_type"))
	instructions := strings.TrimSpace(c.FormValue("instructions"))
	choiceCriteriaStr := c.FormValue("choice_criteria")
	scoreCriteriaStr := c.FormValue("score_criteria")
	threadId := c.FormValue("thread_id")

	if stateStr == "" {
		return utils.Render(c, http.StatusOK, components.ChatMessage("⚠️ Please provide state to evaluate.", false))
	}
	if instructions == "" {
		instructions = "Evaluate this statement"
	}

	var stateData any = stateStr
	var jsonVal any
	if err := json.Unmarshal([]byte(stateStr), &jsonVal); err == nil {
		stateData = jsonVal
	}

	var q jev.Question
	var scaleLevels []string

	switch qType {
	case "noul":
		q = jev.NewNoulQuestion(instructions)
	case "score":
		for _, part := range strings.Split(scoreCriteriaStr, ",") {
			p := strings.TrimSpace(part)
			if p != "" {
				scaleLevels = append(scaleLevels, p)
			}
		}
		if len(scaleLevels) == 0 {
			scaleLevels = []string{"Low priority", "Normal priority", "High urgency"}
		}
		q = jev.NewScoreQuestion(instructions, scaleLevels)
	default:
		qType = "choice"
		criteria := make(map[string]string)
		for _, line := range strings.Split(choiceCriteriaStr, "\n") {
			line = strings.TrimSpace(line)
			if line == "" {
				continue
			}
			parts := strings.SplitN(line, ":", 2)
			if len(parts) == 1 {
				parts = strings.SplitN(line, "=", 2)
			}
			if len(parts) == 2 {
				k := strings.TrimSpace(parts[0])
				v := strings.TrimSpace(parts[1])
				if k != "" {
					criteria[k] = v
				}
			} else {
				criteria[line] = line
			}
		}
		if len(criteria) == 0 {
			criteria = map[string]string{
				"option_a": "First candidate option",
				"option_b": "Second candidate option",
			}
		}
		q = jev.NewChoiceQuestion(instructions, criteria)
	}

	ctx := c.Request().Context()
	res, err := s.jevClient.Evaluate(ctx, stateData, map[string]jev.Question{"q": q})
	if err != nil {
		log.Printf("jev evaluate error: %v", err)
		return utils.Render(c, http.StatusOK, components.ChatMessage("⚠️ Jev evaluation error: "+err.Error(), false))
	}

	ans := res.Answers["q"]

	var probs []components.JevProbability
	for k, v := range ans.Probabilities {
		probs = append(probs, components.JevProbability{Option: k, Prob: v})
	}
	sort.Slice(probs, func(i, j int) bool {
		return probs[i].Prob > probs[j].Prob
	})

	result := components.JevResultData{
		State:        stateStr,
		QuestionType: qType,
		Instructions: instructions,
		Choice:       ans.Choice,
		Confidence:   ans.Confidence,
		Probs:        probs,
		Noul:         ans.Noul,
		Score:        ans.Score,
		ScaleLevels:  scaleLevels,
		InputTokens:  res.Usage.InputTokens,
		OutputTokens: res.Usage.OutputTokens,
		ThreadID:     threadId,
	}

	return utils.Render(c, http.StatusOK, components.JevResult(result))
}

func cleanDescription(s string, original string) string {
	s = regexp.MustCompile(`(?i)\b(dollars?|bucks?)\b`).ReplaceAllString(s, "")
	words := strings.Fields(s)
	cleaned := strings.TrimSpace(strings.Trim(strings.Join(words, " "), "- :,"))
	if cleaned == "" {
		return original
	}
	return cleaned
}

func parseAmountAndDescription(query string) (string, string) {
	// 1. Explicit currency or preposition: $45.50, $45, 45 dollars, 45 bucks, for 32.50, of 32.50
	reExplicit := regexp.MustCompile(`(?i)(?:\$([0-9]+(?:\.[0-9]{1,2})?)|([0-9]+(?:\.[0-9]{1,2})?)\s*(?:dollars?|bucks?|\$)|(?:for|of|-)\s*([0-9]+(?:\.[0-9]{1,2})?))`)
	if match := reExplicit.FindStringSubmatch(query); len(match) > 0 {
		amount := match[1]
		if amount == "" {
			amount = match[2]
		}
		if amount == "" {
			amount = match[3]
		}
		desc := reExplicit.ReplaceAllString(query, "")
		return amount, cleanDescription(desc, query)
	}

	// 2. Any decimal amount (e.g. "Bought groceries 32.50 at Trader Joe's")
	reDecimal := regexp.MustCompile(`\b([0-9]+\.[0-9]{1,2})\b`)
	if match := reDecimal.FindStringSubmatch(query); len(match) > 0 {
		amount := match[1]
		desc := reDecimal.ReplaceAllString(query, "")
		return amount, cleanDescription(desc, query)
	}

	// 3. Number at end of query (e.g. "Lunch 15")
	reEnd := regexp.MustCompile(`(?i)(?:-|\s)+([0-9]+(?:\.[0-9]{1,2})?)\s*$`)
	if match := reEnd.FindStringSubmatch(query); len(match) > 0 {
		amount := match[1]
		desc := query[:len(query)-len(match[0])]
		return amount, cleanDescription(desc, query)
	}

	return "", strings.TrimSpace(query)
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
