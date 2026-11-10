package server

import (
	"net/http"

	"github.com/gokuls-codes/go-gen-ui/internal/utils"
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

	e.GET("/", func(c *echo.Context) error {
		return utils.Render(c, http.StatusOK, pages.Index())
	})

	return e.Start(s.addr)
}
