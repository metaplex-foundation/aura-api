package dynamic

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/labstack/echo/v4"
)

const (
	getUserByIDEndpoint = "https://app.dynamicauth.com/api/v0/users"
)

type Client struct {
	apiToken      string
	environmentID string
	client        *http.Client
}

func NewDynamicClient(apiToken, environmentID string) (*Client, error) {
	return &Client{
		apiToken:      apiToken,
		environmentID: environmentID,
		client: &http.Client{
			Timeout: time.Minute,
		},
	}, nil
}

func (c *Client) getAuthHeader() string {
	return fmt.Sprintf("Bearer %s", c.apiToken)
}

func (c *Client) GetUserByID(ctx context.Context, userID string) (user *User, err error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, fmt.Sprintf("%s/%s", getUserByIDEndpoint, userID), nil)
	if err != nil {
		return user, fmt.Errorf("NewRequestWithContext: %w", err)
	}

	req.Header.Add(echo.HeaderAuthorization, c.getAuthHeader())
	res, err := c.client.Do(req)
	if err != nil {
		return user, fmt.Errorf("do: %w", err)
	}
	defer res.Body.Close()
	body, err := io.ReadAll(res.Body)
	if err != nil {
		return user, fmt.Errorf("ReadAll: %w", err)
	}
	var wrapper UserWrapper
	err = json.Unmarshal(body, &wrapper)
	if err != nil {
		return user, fmt.Errorf("json.Unmarshal: %w", err)
	}

	return &wrapper.User, nil
}
