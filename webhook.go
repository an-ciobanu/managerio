package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
)

// Webhook posts invoice payloads to the teammate's endpoint.
type Webhook struct {
	URL    string
	Client *http.Client
}

// Send POSTs the payload as JSON. Returns an error on transport failure or non-2xx.
func (w Webhook) Send(ctx context.Context, payload WebhookPayload) error {
	body, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("marshal payload: %w", err)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, w.URL, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("build request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := w.Client.Do(req)
	if err != nil {
		return fmt.Errorf("post webhook: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("webhook returned status %d", resp.StatusCode)
	}
	return nil
}
