package main

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestExtractItemsParsesResponse(t *testing.T) {
	// The model returns its structured answer as a JSON string in choices[0].message.content.
	content := `{"items":[{"name":"Piept pui","qty":5,"unit":"kg","unit_price":18.9},{"name":"Ceapa","qty":2,"unit":"kg","unit_price":3.5}]}`
	openAIResp := map[string]any{
		"choices": []map[string]any{
			{"message": map[string]any{"content": content}},
		},
	}
	var gotAuth, gotPath string
	var gotReq map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAuth = r.Header.Get("Authorization")
		gotPath = r.URL.Path
		b, _ := io.ReadAll(r.Body)
		_ = json.Unmarshal(b, &gotReq)
		_ = json.NewEncoder(w).Encode(openAIResp)
	}))
	defer srv.Close()

	v := Vision{BaseURL: srv.URL, APIKey: "test-key", Model: "qwen", Client: srv.Client()}
	items, err := v.ExtractItems(context.Background(), []byte("fake-jpeg-bytes"))
	if err != nil {
		t.Fatalf("ExtractItems: %v", err)
	}
	if len(items) != 2 || items[0].Name != "Piept pui" || items[1].Qty != 2 {
		t.Errorf("parsed items wrong: %+v", items)
	}
	if gotAuth != "Bearer test-key" {
		t.Errorf("auth header = %q", gotAuth)
	}
	if !strings.HasSuffix(gotPath, "/chat/completions") {
		t.Errorf("path = %q, want .../chat/completions", gotPath)
	}
	if gotReq["model"] != "qwen" {
		t.Errorf("model in request = %v, want qwen", gotReq["model"])
	}
}

func TestExtractItemsNon2xx(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
	}))
	defer srv.Close()
	v := Vision{BaseURL: srv.URL, APIKey: "k", Model: "m", Client: srv.Client()}
	if _, err := v.ExtractItems(context.Background(), []byte("x")); err == nil {
		t.Error("expected error on 400, got nil")
	}
}
