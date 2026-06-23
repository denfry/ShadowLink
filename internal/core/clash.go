// SPDX-License-Identifier: GPL-3.0-only
package core

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// ClashClient talks to sing-box's experimental Clash API on loopback.
type ClashClient struct {
	BaseURL string
	Secret  string
	HTTP    *http.Client
}

// NewClashClient builds a client for 127.0.0.1:<port>.
func NewClashClient(port int, secret string) *ClashClient {
	return &ClashClient{
		BaseURL: fmt.Sprintf("http://127.0.0.1:%d", port),
		Secret:  secret,
		HTTP:    &http.Client{Timeout: 30 * time.Second},
	}
}

func (c *ClashClient) auth(r *http.Request) {
	if c.Secret != "" {
		r.Header.Set("Authorization", "Bearer "+c.Secret)
	}
}

// Delay measures latency (ms) for an outbound via GET /proxies/{tag}/delay.
func (c *ClashClient) Delay(ctx context.Context, tag, testURL string, timeoutMS int) (int, error) {
	u := fmt.Sprintf("%s/proxies/%s/delay?timeout=%d&url=%s", c.BaseURL, url.PathEscape(tag), timeoutMS, url.QueryEscape(testURL))
	req, _ := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	c.auth(req)
	resp, err := c.HTTP.Do(req)
	if err != nil {
		return 0, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return 0, fmt.Errorf("delay: status %d", resp.StatusCode)
	}
	var body struct {
		Delay int `json:"delay"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		return 0, err
	}
	return body.Delay, nil
}

// Switch sets the active member of a selector via PUT /proxies/{selector}.
func (c *ClashClient) Switch(ctx context.Context, selector, member string) error {
	u := fmt.Sprintf("%s/proxies/%s", c.BaseURL, url.PathEscape(selector))
	body := strings.NewReader(fmt.Sprintf(`{"name":%q}`, member))
	req, _ := http.NewRequestWithContext(ctx, http.MethodPut, u, body)
	req.Header.Set("Content-Type", "application/json")
	c.auth(req)
	resp, err := c.HTTP.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusNoContent && resp.StatusCode != http.StatusOK {
		return fmt.Errorf("switch: status %d", resp.StatusCode)
	}
	return nil
}
