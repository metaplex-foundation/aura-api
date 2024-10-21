package api

import (
	"fmt"
	"net/http"

	"github.com/gocarina/gocsv"
	"github.com/labstack/echo/v4"
)

const (
	mimeTextCSV = "text/csv"
)

func csvResp(c echo.Context, res interface{}, fileName string) error {
	c.Response().Header().Set(echo.HeaderContentType, mimeTextCSV)
	if fileName != "" {
		c.Response().Header().Set(echo.HeaderContentDisposition, fmt.Sprintf("attachment; filename=%q", fileName))
	}
	c.Response().WriteHeader(http.StatusOK)

	return gocsv.Marshal(res, c.Response())
}

func textResp(c echo.Context, res []byte) error {
	c.Response().Header().Set(echo.HeaderContentType, echo.MIMETextPlainCharsetUTF8)
	c.Response().WriteHeader(http.StatusOK)
	_, err := c.Response().Write(res)

	return err
}
