package utils

import (
	"github.com/a-h/templ"
	"github.com/labstack/echo/v5"
)

func Render(c *echo.Context, statusCode int, t templ.Component) error {
	c.Response().Header().Set("Content-Type", "text/html; charset=utf-8")
	c.Response().WriteHeader(statusCode)
	return t.Render(c.Request().Context(), c.Response())
}
