package server

import (
	"net/http"

	"github.com/gokuls-codes/go-gen-ui/internal/utils"
	"github.com/gokuls-codes/go-gen-ui/internal/views/components"
	"github.com/gokuls-codes/go-gen-ui/internal/views/pages"
	"github.com/labstack/echo/v5"
	"github.com/labstack/echo/v5/middleware"
	"google.golang.org/genai"
)

type Server struct {
	addr     string
	aiClient *genai.Client
}

func NewServer(addr string, aiClient *genai.Client) *Server {
	return &Server{
		addr:     addr,
		aiClient: aiClient,
	}
}

func (s *Server) Start() error {
	e := echo.New()

	e.Use(middleware.RequestLogger())
	e.Use(middleware.Recover())
	e.Static("/static", "static")

	e.GET("/", func(c *echo.Context) error {
		return utils.Render(c, http.StatusOK, pages.Index())
	})

	e.POST("/chat", func(c *echo.Context) error {
		userMessage := c.FormValue("message")
		if userMessage == "" {
			return c.NoContent(http.StatusBadRequest)
		}

		resp, err := s.aiClient.Models.GenerateContent(
			c.Request().Context(),
			"gemini-2.5-flash",
			genai.Text(userMessage),
			nil,
		)
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
