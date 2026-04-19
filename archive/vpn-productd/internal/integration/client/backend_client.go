package client

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"

	v1 "github.com/xtls/xray-core/internal/api/v1"
)

// BackendClient is a generic client for external integrations (e.g. future Telegram bot).
type BackendClient struct {
	baseURL string
	token   string
	client  *http.Client
}

func NewBackendClient(baseURL string, token string, client *http.Client) *BackendClient {
	if client == nil {
		client = http.DefaultClient
	}
	return &BackendClient{
		baseURL: strings.TrimRight(baseURL, "/"),
		token:   token,
		client:  client,
	}
}

func (c *BackendClient) CreateSubscription(ctx context.Context, req v1.CreateSubscriptionRequest) (v1.SubscriptionResponse, error) {
	var out v1.SubscriptionResponse
	if err := c.doJSON(ctx, http.MethodPost, "/admin/subscriptions", req, &out); err != nil {
		return v1.SubscriptionResponse{}, err
	}
	return out, nil
}

func (c *BackendClient) GetSubscription(ctx context.Context, id string) (v1.SubscriptionResponse, error) {
	var out v1.SubscriptionResponse
	if err := c.doJSON(ctx, http.MethodGet, "/admin/subscriptions/"+id, nil, &out); err != nil {
		return v1.SubscriptionResponse{}, err
	}
	return out, nil
}

func (c *BackendClient) RotateSubscription(ctx context.Context, id string) (v1.SubscriptionResponse, error) {
	var out v1.SubscriptionResponse
	if err := c.doJSON(ctx, http.MethodPost, "/admin/subscriptions/"+id+"/rotate", map[string]any{}, &out); err != nil {
		return v1.SubscriptionResponse{}, err
	}
	return out, nil
}

func (c *BackendClient) RevokeSubscription(ctx context.Context, id string) error {
	return c.doJSON(ctx, http.MethodDelete, "/admin/subscriptions/"+id, nil, nil)
}

func (c *BackendClient) doJSON(ctx context.Context, method string, path string, payload any, out any) error {
	var body io.Reader
	if payload != nil {
		raw, err := json.Marshal(payload)
		if err != nil {
			return err
		}
		body = bytes.NewReader(raw)
	}
	req, err := http.NewRequestWithContext(ctx, method, c.baseURL+path, body)
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+c.token)
	req.Header.Set("Content-Type", "application/json")
	resp, err := c.client.Do(req)
	if err != nil {
		return err
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode >= 400 {
		raw, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("backend request failed status=%d body=%s", resp.StatusCode, strings.TrimSpace(string(raw)))
	}
	if out == nil {
		return nil
	}
	return json.NewDecoder(resp.Body).Decode(out)
}

