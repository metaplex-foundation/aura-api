package api

import (
	"context"
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

func (a *api) listenConsul(ctx context.Context) {
	var lastIndex uint64
	for {
		select {
		case <-ctx.Done():
			return
		default:
		}
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
			var pricing PricingPlans
			if err = json.Unmarshal(pair.Value, &pricing); err != nil {
				log.Logger.API.Errorf("listenConsul: json.Unmarshal: %s", err)
				continue
			}

			a.pricing = pricing
			lastIndex = meta.LastIndex
		}
	}
}
