package api

import (
	_ "embed"
	"net/http"

	"github.com/labstack/echo/v4"
)

var (
	//go:embed index.html
	index string
	//go:embed api.yaml
	apiSpec string
)

func ServeDocs(e *echo.Echo) {
	e.GET("/docs", func(c echo.Context) error {
		return c.HTML(http.StatusOK, index)
	})
	// serve API spec
	e.GET("/openapi.yaml", func(c echo.Context) error {
		return c.Blob(http.StatusOK, "application/x-yaml", []byte(apiSpec))
	})
}
