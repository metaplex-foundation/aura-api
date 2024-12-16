package api

import (
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/gocarina/gocsv"
	consulAPI "github.com/hashicorp/consul/api"
	"github.com/labstack/echo/v4"

	"github.com/adm-metaex/aura-api/pkg/log"
)

const (
	mimeTextCSV = "text/csv"
)
const consulPricingPath = "aura-api/config/pricing"

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

func (a *api) listenConsul() {
	var lastIndex uint64 = 0
	for {
		queryOpts := &consulAPI.QueryOptions{
			WaitIndex: lastIndex,
			WaitTime:  time.Minute,
		}
		pair, meta, err := a.consulKV.Get(consulPricingPath, queryOpts)
		if err != nil {
			log.Logger.API.Errorf("listenConsul: consulKV.Get: %s", err)
			continue
		}

		if pair != nil && meta.LastIndex > lastIndex {
			var pricing pricingConfig
			if err = json.Unmarshal(pair.Value, &pricing); err != nil {
				log.Logger.API.Errorf("listenConsul: json.Unmarshal: %s", err)
				continue
			}

			a.pricing = pricing
			lastIndex = meta.LastIndex
		}
	}
}
