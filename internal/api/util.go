package api

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/gagliardetto/solana-go"
	"github.com/gocarina/gocsv"
	consulAPI "github.com/hashicorp/consul/api"
	"github.com/labstack/echo/v4"
	"github.com/shopspring/decimal"

	"github.com/adm-metaex/aura-api/pkg/log"
)

const (
	mimeTextCSV = "text/csv"
)
const (
	consulPricingPath           = "config/aura-api/pricing"
	consulMplxPricePath         = "config/aura-api/mplx"
	consulPaymentsRecipientPath = "config/aura-api/payments/recipient"
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

func (a *api) listenConsul(ctx context.Context) {
	go func() {
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
				log.Logger.API.Errorf("listenConsul: consulKV.Get %s: %s", consulPricingPath, err)
				continue
			}

			if pair != nil && meta.LastIndex > lastIndex {
				var pricing PricingPlans
				if err = json.Unmarshal(pair.Value, &pricing); err != nil {
					log.Logger.API.Errorf("listenConsul: json.Unmarshal %s: %s", consulPricingPath, err)
					continue
				}

				a.pricing = pricing
				lastIndex = meta.LastIndex
				log.Logger.API.Infof("New pricing config received: %+v", pricing)
			}
		}
	}()

	go func() {
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
			pair, meta, err := a.consulKV.Get(consulMplxPricePath, queryOpts)
			if err != nil {
				log.Logger.API.Errorf("listenConsul: consulKV.Get %s: %s", consulMplxPricePath, err)
				continue
			}

			if pair != nil && meta.LastIndex > lastIndex {
				price, err := decimal.NewFromString(string(pair.Value))
				if err != nil {
					log.Logger.API.Errorf("listenConsul: NewFromString: %s", err)
					lastIndex = meta.LastIndex
					continue
				}

				a.mplxPrice = price
				lastIndex = meta.LastIndex
				log.Logger.API.Infof("New MPLX price received: %s", price.String())
			}
		}
	}()

	go func() {
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
			pair, meta, err := a.consulKV.Get(consulPaymentsRecipientPath, queryOpts)
			if err != nil {
				log.Logger.API.Errorf("listenConsul: consulKV.Get %s: %s", consulPaymentsRecipientPath, err)
				continue
			}

			if pair != nil && meta.LastIndex > lastIndex {
				newRecipient, err := solana.PublicKeyFromBase58(string(pair.Value))
				if err != nil {
					log.Logger.API.Errorf("listenConsul: PublicKeyFromBase58: %s", err)
					lastIndex = meta.LastIndex
					continue
				}
				newpaymentRecipientAssociatedTokenAddress, _, err := solana.FindAssociatedTokenAddress(newRecipient, metaplexToken)
				if err != nil {
					log.Logger.API.Errorf("listenConsul: FindAssociatedTokenAddress: %s", err)
					lastIndex = meta.LastIndex
					continue
				}
				a.paymentRecipient = newRecipient
				a.paymentWatcher.paymentRecipient = newRecipient
				a.paymentWatcher.paymentRecipientAssociatedTokenAddress = newpaymentRecipientAssociatedTokenAddress
				lastIndex = meta.LastIndex
				log.Logger.API.Infof("New payment recipient received: %s", newRecipient.String())
			}
		}
	}()
}
