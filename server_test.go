package main

import (
	"bytes"
	"context"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// newMultipartRequest builds a POST /scan request carrying one image part.
func newMultipartRequest(t *testing.T, fieldData []byte) *http.Request {
	t.Helper()
	var buf bytes.Buffer
	mw := multipart.NewWriter(&buf)
	fw, err := mw.CreateFormFile("photo", "invoice.jpg")
	if err != nil {
		t.Fatal(err)
	}
	fw.Write(fieldData)
	mw.Close()
	req := httptest.NewRequest(http.MethodPost, "/scan", &buf)
	req.Header.Set("Content-Type", mw.FormDataContentType())
	return req
}

func newTestServer(t *testing.T, extract func(context.Context, []byte) ([]Item, error), send func(context.Context, WebhookPayload) error) *Server {
	t.Helper()
	s, err := NewServer(Config{RestaurantID: "demo"}, extract, send)
	if err != nil {
		t.Fatalf("NewServer: %v", err)
	}
	return s
}

func TestHandleScanSuccess(t *testing.T) {
	extract := func(_ context.Context, _ []byte) ([]Item, error) {
		return []Item{{Name: "Piept pui", Qty: 5, Unit: "kg", UnitPrice: 18.9}}, nil
	}
	var sent WebhookPayload
	send := func(_ context.Context, p WebhookPayload) error { sent = p; return nil }
	s := newTestServer(t, extract, send)

	rec := httptest.NewRecorder()
	s.handleScan(rec, newMultipartRequest(t, []byte("img")))

	body := rec.Body.String()
	if !strings.Contains(body, "Piept pui") {
		t.Errorf("response missing item name: %s", body)
	}
	if !strings.Contains(body, "Trimis") {
		t.Errorf("response missing success status: %s", body)
	}
	if sent.RestaurantID != "demo" || len(sent.Items) != 1 {
		t.Errorf("webhook payload wrong: %+v", sent)
	}
}

func TestHandleScanExtractError(t *testing.T) {
	extract := func(_ context.Context, _ []byte) ([]Item, error) {
		return nil, context.DeadlineExceeded
	}
	sendCalled := false
	send := func(_ context.Context, _ WebhookPayload) error { sendCalled = true; return nil }
	s := newTestServer(t, extract, send)

	rec := httptest.NewRecorder()
	s.handleScan(rec, newMultipartRequest(t, []byte("img")))

	if sendCalled {
		t.Error("webhook should NOT be called when extraction fails")
	}
	if !strings.Contains(rec.Body.String(), "analiza") {
		t.Errorf("expected analysis-error message, got: %s", rec.Body.String())
	}
}

func TestHandleScanWebhookError(t *testing.T) {
	extract := func(_ context.Context, _ []byte) ([]Item, error) {
		return []Item{{Name: "Ceapa", Qty: 2, Unit: "kg", UnitPrice: 3.5}}, nil
	}
	send := func(_ context.Context, _ WebhookPayload) error { return context.DeadlineExceeded }
	s := newTestServer(t, extract, send)

	rec := httptest.NewRecorder()
	s.handleScan(rec, newMultipartRequest(t, []byte("img")))

	body := rec.Body.String()
	if !strings.Contains(body, "Ceapa") {
		t.Errorf("parsed items should still be shown: %s", body)
	}
	if !strings.Contains(body, "eșuat") {
		t.Errorf("expected send-failed note: %s", body)
	}
}

func TestHandleIndexServesPage(t *testing.T) {
	s := newTestServer(t,
		func(_ context.Context, _ []byte) ([]Item, error) { return nil, nil },
		func(_ context.Context, _ WebhookPayload) error { return nil },
	)
	rec := httptest.NewRecorder()
	s.handleIndex(rec, httptest.NewRequest(http.MethodGet, "/", nil))
	if !strings.Contains(rec.Body.String(), "Scanează") {
		t.Errorf("index page missing heading: %s", rec.Body.String())
	}
}
