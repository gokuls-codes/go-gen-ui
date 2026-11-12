package utils

import (
	"bytes"
	"context"

	"github.com/a-h/templ"
	"github.com/labstack/echo/v5"
)

func Render(c *echo.Context, statusCode int, t templ.Component) error {
	c.Response().Header().Set("Content-Type", "text/html; charset=utf-8")
	c.Response().WriteHeader(statusCode)
	return t.Render(c.Request().Context(), c.Response())
}

// RenderToString renders any templ.Component into an HTML string.
func RenderToString(ctx context.Context, t templ.Component) (string, error) {
	var buf bytes.Buffer
	if err := t.Render(ctx, &buf); err != nil {
		return "", err
	}
	return buf.String(), nil
}
