package main

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestWebhookSendSuccess(t *testing.T) {
	var gotMethod, gotCT string
	var gotBody WebhookPayload
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotMethod = r.Method
		gotCT = r.Header.Get("Content-Type")
		b, _ := io.ReadAll(r.Body)
		_ = json.Unmarshal(b, &gotBody)
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	wh := Webhook{URL: srv.URL, Client: srv.Client()}
	payload := WebhookPayload{RestaurantID: "demo", Items: []Item{{Name: "Ceapa", Qty: 2, Unit: "kg", UnitPrice: 3.5}}}
	if err := wh.Send(context.Background(), payload); err != nil {
		t.Fatalf("Send: %v", err)
	}
	if gotMethod != http.MethodPost {
		t.Errorf("method = %s, want POST", gotMethod)
	}
	if gotCT != "application/json" {
		t.Errorf("content-type = %s", gotCT)
	}
	if len(gotBody.Items) != 1 || gotBody.Items[0].Name != "Ceapa" {
		t.Errorf("body not received correctly: %+v", gotBody)
	}
}

func TestWebhookSendNon2xx(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()

	wh := Webhook{URL: srv.URL, Client: srv.Client()}
	if err := wh.Send(context.Background(), WebhookPayload{}); err == nil {
		t.Error("expected error on 500, got nil")
	}
}
