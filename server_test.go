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

// newMultipartRequest builds a POST /scan request carrying the kind field + one image part.
func newMultipartRequest(t *testing.T, kind string, fieldData []byte) *http.Request {
	t.Helper()
	var buf bytes.Buffer
	mw := multipart.NewWriter(&buf)
	if err := mw.WriteField("kind", kind); err != nil {
		t.Fatal(err)
	}
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

func newTestServer(t *testing.T, extract func(context.Context, []byte, bool) ([]Item, error), send func(context.Context, WebhookPayload) error) *Server {
	t.Helper()
	s, err := NewServer(Config{RestaurantID: "demo"}, extract, send)
	if err != nil {
		t.Fatalf("NewServer: %v", err)
	}
	return s
}

func TestHandleScanInvoiceSuccess(t *testing.T) {
	var gotInput bool
	extract := func(_ context.Context, _ []byte, input bool) ([]Item, error) {
		gotInput = input
		return []Item{{Name: "Piept pui", Qty: 5, Unit: "kg", UnitPrice: 18.9}}, nil
	}
	var sent WebhookPayload
	send := func(_ context.Context, p WebhookPayload) error { sent = p; return nil }
	s := newTestServer(t, extract, send)

	rec := httptest.NewRecorder()
	s.handleScan(rec, newMultipartRequest(t, "in", []byte("img")))

	body := rec.Body.String()
	if !strings.Contains(body, "Piept pui") {
		t.Errorf("response missing item name: %s", body)
	}
	if !strings.Contains(body, "Intrare") {
		t.Errorf("response missing direction label: %s", body)
	}
	if !gotInput {
		t.Error("extract should be called with input=true for kind=in")
	}
	if sent.RestaurantID != "demo" || !sent.Input || len(sent.Items) != 1 {
		t.Errorf("webhook payload wrong: %+v", sent)
	}
}

func TestHandleScanBonFiscalSetsInputFalse(t *testing.T) {
	gotInput := true
	extract := func(_ context.Context, _ []byte, input bool) ([]Item, error) {
		gotInput = input
		return []Item{{Name: "Meniu zilei", Qty: 2, Unit: "buc", UnitPrice: 25}}, nil
	}
	var sent WebhookPayload
	send := func(_ context.Context, p WebhookPayload) error { sent = p; return nil }
	s := newTestServer(t, extract, send)

	rec := httptest.NewRecorder()
	s.handleScan(rec, newMultipartRequest(t, "out", []byte("img")))

	if gotInput {
		t.Error("extract should be called with input=false for kind=out")
	}
	if sent.Input {
		t.Errorf("payload Input should be false for bon fiscal: %+v", sent)
	}
	if !strings.Contains(rec.Body.String(), "Ieșire") {
		t.Errorf("response missing Ieșire label: %s", rec.Body.String())
	}
}

func TestHandleScanUnknownKind(t *testing.T) {
	extractCalled, sendCalled := false, false
	extract := func(_ context.Context, _ []byte, _ bool) ([]Item, error) {
		extractCalled = true
		return []Item{{Name: "x"}}, nil
	}
	send := func(_ context.Context, _ WebhookPayload) error { sendCalled = true; return nil }
	s := newTestServer(t, extract, send)

	rec := httptest.NewRecorder()
	s.handleScan(rec, newMultipartRequest(t, "", []byte("img")))

	if extractCalled || sendCalled {
		t.Error("neither extract nor webhook should run for an unknown kind")
	}
	if !strings.Contains(rec.Body.String(), "necunoscut") {
		t.Errorf("expected unknown-kind error, got: %s", rec.Body.String())
	}
}

func TestHandleScanExtractError(t *testing.T) {
	extract := func(_ context.Context, _ []byte, _ bool) ([]Item, error) {
		return nil, context.DeadlineExceeded
	}
	sendCalled := false
	send := func(_ context.Context, _ WebhookPayload) error { sendCalled = true; return nil }
	s := newTestServer(t, extract, send)

	rec := httptest.NewRecorder()
	s.handleScan(rec, newMultipartRequest(t, "in", []byte("img")))

	if sendCalled {
		t.Error("webhook should NOT be called when extraction fails")
	}
	if !strings.Contains(rec.Body.String(), "analiza") {
		t.Errorf("expected analysis-error message, got: %s", rec.Body.String())
	}
}

func TestHandleScanWebhookError(t *testing.T) {
	extract := func(_ context.Context, _ []byte, _ bool) ([]Item, error) {
		return []Item{{Name: "Ceapa", Qty: 2, Unit: "kg", UnitPrice: 3.5}}, nil
	}
	send := func(_ context.Context, _ WebhookPayload) error { return context.DeadlineExceeded }
	s := newTestServer(t, extract, send)

	rec := httptest.NewRecorder()
	s.handleScan(rec, newMultipartRequest(t, "in", []byte("img")))

	body := rec.Body.String()
	if !strings.Contains(body, "Ceapa") {
		t.Errorf("parsed items should still be shown: %s", body)
	}
	if !strings.Contains(body, "eșuat") {
		t.Errorf("expected send-failed note: %s", body)
	}
}

func TestHandleIndexServesBothButtons(t *testing.T) {
	s := newTestServer(t,
		func(_ context.Context, _ []byte, _ bool) ([]Item, error) { return nil, nil },
		func(_ context.Context, _ WebhookPayload) error { return nil },
	)
	rec := httptest.NewRecorder()
	s.handleIndex(rec, httptest.NewRequest(http.MethodGet, "/", nil))
	body := rec.Body.String()
	if !strings.Contains(body, "Factură furnizor") || !strings.Contains(body, "Bon fiscal") {
		t.Errorf("index page missing in/out buttons: %s", body)
	}
}
