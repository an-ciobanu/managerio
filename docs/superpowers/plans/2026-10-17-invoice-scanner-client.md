# Invoice Scanner Client Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** A Go web service that lets a Romanian restaurant owner photograph a Metro/Selgros invoice on their phone, extracts the line items with a vision LLM, and POSTs them as JSON to a teammate's webhook.

**Architecture:** Standard-library `net/http` server rendering SSR HTML with HTMX (no custom JS, no framework). The phone camera is reached with a plain `<input type="file" capture>`. Two seams hide all external I/O: `ExtractItems()` (vision LLM over an OpenAI-compatible API — Fireworks default, DeepInfra swappable by env) and `SendToWebhook()`. Handlers depend only on those two functions, so they test with fakes and never touch the network.

**Tech Stack:** Go 1.22+ (`net/http.ServeMux` method-pattern routing, `embed` for templates, `html/template`), HTMX 2 from CDN, Qwen2.5-VL via Fireworks/DeepInfra OpenAI-compatible endpoint, Caddy for TLS on Hetzner.

> **🔑 Secrets you'll need (I will never ask you to paste them into chat or a committed file):**
> - **Task 5 / Task 8 — `VISION_API_KEY`** (Fireworks, and optionally DeepInfra): first needed when you run the service against a real invoice photo.
> - **Task 9 — Hetzner SSH access + Cloudflare DNS**: needed only at deploy time; *you* run those steps, I supply the commands. I never handle the keys.
> All secrets are read from environment variables / a gitignored `.env`, never hardcoded.

---

## File Structure

| File | Responsibility |
|------|----------------|
| `go.mod` | Module definition, Go version |
| `types.go` | `Item`, `WebhookPayload`, `resultView` — the data contract (the one place to change the webhook JSON) |
| `config.go` | `Config` struct + `LoadConfig()` reading env vars with defaults |
| `webhook.go` | `Webhook` client + `Send()` — POST JSON to the teammate's webhook |
| `vision.go` | `Vision` client + `ExtractItems()` — OpenAI-compatible vision call, provider hidden |
| `server.go` | `Server` struct, template embedding, `handleIndex` / `handleScan` / render helpers |
| `main.go` | `LoadConfig`, build real clients, wire routes, start server |
| `templates/index.html` | Upload page (HTMX + minimal CSS) |
| `templates/fragment.html` | Result fragment (items table + status) returned by `/scan` |
| `*_test.go` | Table/httptest tests alongside each file |
| `Caddyfile`, `scanner.service` | Deploy artifacts (Task 9) |

---

## Task 1: Initialize Go module

**Files:**
- Create: `go.mod`
- Create: `main.go`

- [ ] **Step 1: Initialize the module**

Run:
```bash
go mod init managerio/scanner
```
Expected: creates `go.mod` containing `module managerio/scanner` and a `go 1.2x` line.

- [ ] **Step 2: Ensure the Go version supports method-pattern routing**

Open `go.mod` and confirm the `go` directive is `1.22` or higher. If it is lower, change that line to:
```
go 1.22
```

- [ ] **Step 3: Create a minimal buildable main**

Create `main.go`:
```go
package main

func main() {}
```

- [ ] **Step 4: Verify it builds**

Run: `go build ./...`
Expected: no output, exit 0.

- [ ] **Step 5: Commit**

```bash
git add go.mod main.go
git commit -m "chore: initialize Go module"
```

---

## Task 2: Data contract (types.go)

**Files:**
- Create: `types.go`
- Test: `types_test.go`

- [ ] **Step 1: Write the failing test**

Create `types_test.go`:
```go
package main

import (
	"encoding/json"
	"testing"
)

func TestWebhookPayloadJSON(t *testing.T) {
	p := WebhookPayload{
		RestaurantID: "demo",
		Source:       "metro",
		Items:        []Item{{Name: "Piept pui", Qty: 5, Unit: "kg", UnitPrice: 18.9}},
	}
	b, err := json.Marshal(p)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	got := string(b)
	want := `{"restaurant_id":"demo","source":"metro","items":[{"name":"Piept pui","qty":5,"unit":"kg","unit_price":18.9}]}`
	if got != want {
		t.Errorf("got  %s\nwant %s", got, want)
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./... -run TestWebhookPayloadJSON`
Expected: FAIL — `undefined: WebhookPayload` / `undefined: Item`.

- [ ] **Step 3: Write the types**

Create `types.go`:
```go
package main

// Item is one line on an invoice.
type Item struct {
	Name      string  `json:"name"`
	Qty       float64 `json:"qty"`
	Unit      string  `json:"unit"` // e.g. "kg", "buc", "l"
	UnitPrice float64 `json:"unit_price"`
}

// WebhookPayload is the body POSTed to the teammate's webhook.
// This is the single place to change the webhook contract.
type WebhookPayload struct {
	RestaurantID string `json:"restaurant_id"`
	Source       string `json:"source"`
	Items        []Item `json:"items"`
}

// resultView is the data passed to templates/fragment.html.
type resultView struct {
	Items      []Item
	SendFailed bool
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./... -run TestWebhookPayloadJSON`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add types.go types_test.go
git commit -m "feat: add Item and WebhookPayload data contract"
```

---

## Task 3: Configuration (config.go)

**Files:**
- Create: `config.go`
- Test: `config_test.go`

- [ ] **Step 1: Write the failing test**

Create `config_test.go`:
```go
package main

import "testing"

func TestLoadConfigDefaults(t *testing.T) {
	// No env vars set for optional values -> defaults apply.
	cfg := LoadConfig()
	if cfg.Port != "8080" {
		t.Errorf("Port default = %q, want 8080", cfg.Port)
	}
	if cfg.RestaurantID != "demo" {
		t.Errorf("RestaurantID default = %q, want demo", cfg.RestaurantID)
	}
	if cfg.VisionBaseURL == "" {
		t.Error("VisionBaseURL default should not be empty")
	}
}

func TestLoadConfigOverrides(t *testing.T) {
	t.Setenv("PORT", "9000")
	t.Setenv("VISION_MODEL", "custom-model")
	t.Setenv("WEBHOOK_URL", "http://localhost:7000/webhook")
	cfg := LoadConfig()
	if cfg.Port != "9000" {
		t.Errorf("Port = %q, want 9000", cfg.Port)
	}
	if cfg.VisionModel != "custom-model" {
		t.Errorf("VisionModel = %q, want custom-model", cfg.VisionModel)
	}
	if cfg.WebhookURL != "http://localhost:7000/webhook" {
		t.Errorf("WebhookURL = %q", cfg.WebhookURL)
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./... -run TestLoadConfig`
Expected: FAIL — `undefined: LoadConfig`.

- [ ] **Step 3: Write the config loader**

Create `config.go`:
```go
package main

import "os"

// Config holds all runtime configuration, sourced from environment variables.
type Config struct {
	Port          string
	VisionBaseURL string
	VisionAPIKey  string
	VisionModel   string
	WebhookURL    string
	RestaurantID  string
}

// LoadConfig reads configuration from the environment, applying defaults.
func LoadConfig() Config {
	return Config{
		Port:          env("PORT", "8080"),
		VisionBaseURL: env("VISION_BASE_URL", "https://api.fireworks.ai/inference/v1"),
		VisionAPIKey:  env("VISION_API_KEY", ""),
		VisionModel:   env("VISION_MODEL", "accounts/fireworks/models/qwen2p5-vl-72b-instruct"),
		WebhookURL:    env("WEBHOOK_URL", ""),
		RestaurantID:  env("RESTAURANT_ID", "demo"),
	}
}

func env(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}
```

> Note: the default `VISION_MODEL` slug and `VISION_BASE_URL` are Fireworks values and are **env-overridable**. Confirm the exact Qwen2.5-VL slug in your Fireworks console in Task 8; if it differs, set `VISION_MODEL` rather than editing code. DeepInfra equivalents (Task 8): `VISION_BASE_URL=https://api.deepinfra.com/v1/openai`, `VISION_MODEL=Qwen/Qwen2.5-VL-72B-Instruct`.

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./... -run TestLoadConfig`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add config.go config_test.go
git commit -m "feat: add env-based configuration"
```

---

## Task 4: Webhook client (webhook.go)

**Files:**
- Create: `webhook.go`
- Test: `webhook_test.go`

- [ ] **Step 1: Write the failing test**

Create `webhook_test.go`:
```go
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
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./... -run TestWebhook`
Expected: FAIL — `undefined: Webhook`.

- [ ] **Step 3: Write the webhook client**

Create `webhook.go`:
```go
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
```

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./... -run TestWebhook`
Expected: PASS (both subtests).

- [ ] **Step 5: Commit**

```bash
git add webhook.go webhook_test.go
git commit -m "feat: add webhook client"
```

---

## Task 5: Vision client (vision.go)

**Files:**
- Create: `vision.go`
- Test: `vision_test.go`

This talks to an OpenAI-compatible `/chat/completions` endpoint. The test injects a fake server via `BaseURL`, so no real API key is needed to pass the tests. The real key is used in Task 8.

- [ ] **Step 1: Write the failing test**

Create `vision_test.go`:
```go
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
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./... -run TestExtractItems`
Expected: FAIL — `undefined: Vision`.

- [ ] **Step 3: Write the vision client**

Create `vision.go`:
```go
package main

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
)

// Vision extracts invoice items from an image using an OpenAI-compatible
// vision model (Qwen2.5-VL on Fireworks or DeepInfra). The provider is hidden
// behind BaseURL/APIKey/Model, all of which come from config.
type Vision struct {
	BaseURL string
	APIKey  string
	Model   string
	Client  *http.Client
}

const visionPrompt = `You are reading a Romanian supplier invoice from Metro or Selgros.
Extract every product line item. For each item return:
- name: the product name exactly as printed (Romanian)
- qty: the quantity as a number
- unit: the unit of measure (e.g. kg, buc, l)
- unit_price: the price per unit as a number
Return only the structured data as JSON matching the schema.`

// invoiceSchema constrains the model output to {"items":[{name,qty,unit,unit_price}]}.
var invoiceSchema = map[string]any{
	"type": "object",
	"properties": map[string]any{
		"items": map[string]any{
			"type": "array",
			"items": map[string]any{
				"type": "object",
				"properties": map[string]any{
					"name":       map[string]any{"type": "string"},
					"qty":        map[string]any{"type": "number"},
					"unit":       map[string]any{"type": "string"},
					"unit_price": map[string]any{"type": "number"},
				},
				"required": []string{"name", "qty", "unit", "unit_price"},
			},
		},
	},
	"required": []string{"items"},
}

type chatReq struct {
	Model          string      `json:"model"`
	MaxTokens      int         `json:"max_tokens"`
	Messages       []chatMsg   `json:"messages"`
	ResponseFormat respFormat  `json:"response_format"`
}

type chatMsg struct {
	Role    string        `json:"role"`
	Content []contentPart `json:"content"`
}

type contentPart struct {
	Type     string    `json:"type"`
	Text     string    `json:"text,omitempty"`
	ImageURL *imageURL `json:"image_url,omitempty"`
}

type imageURL struct {
	URL string `json:"url"`
}

type respFormat struct {
	Type       string     `json:"type"`
	JSONSchema schemaSpec `json:"json_schema"`
}

type schemaSpec struct {
	Name   string         `json:"name"`
	Schema map[string]any `json:"schema"`
}

type chatResp struct {
	Choices []struct {
		Message struct {
			Content string `json:"content"`
		} `json:"message"`
	} `json:"choices"`
}

type itemsWrapper struct {
	Items []Item `json:"items"`
}

// ExtractItems sends the image to the vision model and returns the parsed line items.
func (v Vision) ExtractItems(ctx context.Context, img []byte) ([]Item, error) {
	dataURI := "data:image/jpeg;base64," + base64.StdEncoding.EncodeToString(img)
	reqBody := chatReq{
		Model:     v.Model,
		MaxTokens: 4096,
		Messages: []chatMsg{{
			Role: "user",
			Content: []contentPart{
				{Type: "text", Text: visionPrompt},
				{Type: "image_url", ImageURL: &imageURL{URL: dataURI}},
			},
		}},
		ResponseFormat: respFormat{
			Type:       "json_schema",
			JSONSchema: schemaSpec{Name: "invoice_items", Schema: invoiceSchema},
		},
	}
	body, err := json.Marshal(reqBody)
	if err != nil {
		return nil, fmt.Errorf("marshal request: %w", err)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, v.BaseURL+"/chat/completions", bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("build request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+v.APIKey)
	resp, err := v.Client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("call vision api: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("vision api status %d", resp.StatusCode)
	}
	var cr chatResp
	if err := json.NewDecoder(resp.Body).Decode(&cr); err != nil {
		return nil, fmt.Errorf("decode response: %w", err)
	}
	if len(cr.Choices) == 0 {
		return nil, fmt.Errorf("vision api returned no choices")
	}
	var wrapper itemsWrapper
	if err := json.Unmarshal([]byte(cr.Choices[0].Message.Content), &wrapper); err != nil {
		return nil, fmt.Errorf("parse items json: %w", err)
	}
	return wrapper.Items, nil
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./... -run TestExtractItems`
Expected: PASS (both subtests).

- [ ] **Step 5: Commit**

```bash
git add vision.go vision_test.go
git commit -m "feat: add vision client (OpenAI-compatible, provider-agnostic)"
```

---

## Task 6: Server, handlers, and templates (server.go + templates)

**Files:**
- Create: `server.go`
- Create: `templates/index.html`
- Create: `templates/fragment.html`
- Test: `server_test.go`

The `Server` holds the two seams as function values so tests inject fakes.

- [ ] **Step 1: Create the templates**

Create `templates/index.html`:
```html
<!doctype html>
<html lang="ro">
<head>
  <meta charset="utf-8">
  <meta name="viewport" content="width=device-width, initial-scale=1">
  <title>Scanare Factură</title>
  <script src="https://unpkg.com/htmx.org@2.0.3"></script>
  <style>
    body { font-family: system-ui, sans-serif; max-width: 480px; margin: 0 auto; padding: 16px; color: #111; }
    h1 { font-size: 1.25rem; }
    .btn { display: block; width: 100%; padding: 14px; font-size: 1rem; border: 0; border-radius: 8px; background: #2563eb; color: #fff; }
    table { width: 100%; border-collapse: collapse; margin-top: 12px; }
    th, td { text-align: left; padding: 6px; border-bottom: 1px solid #eee; font-size: 0.9rem; }
    .ok { color: #16a34a; } .err { color: #dc2626; }
    .htmx-indicator { display: none; margin-top: 12px; }
    .htmx-request .htmx-indicator, .htmx-request.htmx-indicator { display: block; }
  </style>
</head>
<body>
  <h1>Scanează factura Metro / Selgros</h1>
  <form hx-post="/scan" hx-target="#result" hx-encoding="multipart/form-data" hx-indicator="#ind">
    <input class="btn" type="file" name="photo" accept="image/*" capture="environment"
           onchange="this.form.requestSubmit()">
  </form>
  <div id="ind" class="htmx-indicator">Analizez factura…</div>
  <div id="result"></div>
</body>
</html>
```

Create `templates/fragment.html`:
```html
{{if .SendFailed}}
<p class="err">Factura a fost citită, dar trimiterea către server a eșuat. Încearcă din nou.</p>
{{else}}
<p class="ok">Trimis ✓ ({{len .Items}} produse)</p>
{{end}}
<table>
  <tr><th>Produs</th><th>Cant.</th><th>UM</th><th>Preț</th></tr>
  {{range .Items}}
  <tr><td>{{.Name}}</td><td>{{.Qty}}</td><td>{{.Unit}}</td><td>{{printf "%.2f" .UnitPrice}}</td></tr>
  {{end}}
</table>
```

- [ ] **Step 2: Write the failing test**

Create `server_test.go`:
```go
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
```

- [ ] **Step 3: Run test to verify it fails**

Run: `go test ./... -run TestHandle`
Expected: FAIL — `undefined: NewServer` / `undefined: Server`.

- [ ] **Step 4: Write the server**

Create `server.go`:
```go
package main

import (
	"context"
	"embed"
	"html/template"
	"io"
	"net/http"
)

//go:embed templates/*.html
var templatesFS embed.FS

const maxUploadBytes = 10 << 20 // 10 MB

// Server holds config, the external seams (extract/send), and parsed templates.
type Server struct {
	cfg       Config
	extract   func(ctx context.Context, img []byte) ([]Item, error)
	send      func(ctx context.Context, payload WebhookPayload) error
	templates *template.Template
}

// NewServer parses templates and wires the dependencies.
func NewServer(cfg Config,
	extract func(ctx context.Context, img []byte) ([]Item, error),
	send func(ctx context.Context, payload WebhookPayload) error,
) (*Server, error) {
	tmpl, err := template.ParseFS(templatesFS, "templates/*.html")
	if err != nil {
		return nil, err
	}
	return &Server{cfg: cfg, extract: extract, send: send, templates: tmpl}, nil
}

// Routes returns the configured HTTP handler.
func (s *Server) Routes() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /", s.handleIndex)
	mux.HandleFunc("POST /scan", s.handleScan)
	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("ok"))
	})
	return mux
}

func (s *Server) handleIndex(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := s.templates.ExecuteTemplate(w, "index.html", nil); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

func (s *Server) handleScan(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, maxUploadBytes)
	if err := r.ParseMultipartForm(maxUploadBytes); err != nil {
		s.renderError(w, "Fișier prea mare sau invalid. Încearcă din nou.")
		return
	}
	file, _, err := r.FormFile("photo")
	if err != nil {
		s.renderError(w, "Nicio imagine primită. Fă o poză facturii.")
		return
	}
	defer file.Close()
	img, err := io.ReadAll(file)
	if err != nil || len(img) == 0 {
		s.renderError(w, "Nu am putut citi imaginea. Încearcă din nou.")
		return
	}

	items, err := s.extract(r.Context(), img)
	if err != nil || len(items) == 0 {
		s.renderError(w, "Nu am putut analiza factura. Încearcă o poză mai clară.")
		return
	}

	payload := WebhookPayload{RestaurantID: s.cfg.RestaurantID, Items: items}
	sendErr := s.send(r.Context(), payload)

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	_ = s.templates.ExecuteTemplate(w, "fragment.html", resultView{Items: items, SendFailed: sendErr != nil})
}

func (s *Server) renderError(w http.ResponseWriter, msg string) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	_, _ = w.Write([]byte(`<p class="err">` + template.HTMLEscapeString(msg) + `</p>`))
}
```

- [ ] **Step 5: Run test to verify it passes**

Run: `go test ./... -run TestHandle`
Expected: PASS (all four subtests).

- [ ] **Step 6: Run the full suite**

Run: `go test ./...`
Expected: PASS (all tests from Tasks 2–6).

- [ ] **Step 7: Commit**

```bash
git add server.go templates/index.html templates/fragment.html server_test.go
git commit -m "feat: add HTTP server, handlers, and HTMX templates"
```

---

## Task 7: Wire main.go and run locally

**Files:**
- Modify: `main.go`

- [ ] **Step 1: Replace main.go with the real wiring**

Replace the contents of `main.go` with:
```go
package main

import (
	"log"
	"net/http"
	"time"
)

func main() {
	cfg := LoadConfig()

	vision := Vision{
		BaseURL: cfg.VisionBaseURL,
		APIKey:  cfg.VisionAPIKey,
		Model:   cfg.VisionModel,
		Client:  &http.Client{Timeout: 60 * time.Second},
	}
	webhook := Webhook{
		URL:    cfg.WebhookURL,
		Client: &http.Client{Timeout: 10 * time.Second},
	}

	srv, err := NewServer(cfg, vision.ExtractItems, webhook.Send)
	if err != nil {
		log.Fatalf("init server: %v", err)
	}

	log.Printf("listening on :%s", cfg.Port)
	log.Fatal(http.ListenAndServe(":"+cfg.Port, srv.Routes()))
}
```

- [ ] **Step 2: Verify it builds and the suite still passes**

Run: `go build ./... && go test ./...`
Expected: build succeeds, all tests PASS.

- [ ] **Step 3: Smoke-test the server locally (no key needed)**

Run in one terminal:
```bash
PORT=8080 go run .
```
In another terminal:
```bash
curl -s localhost:8080/healthz          # expect: ok
curl -s localhost:8080/ | grep -o "Scanează"   # expect: Scanează
```
Stop the server with Ctrl-C.

- [ ] **Step 4: Commit**

```bash
git add main.go
git commit -m "feat: wire config, clients, and routes in main"
```

---

## Task 8: Live end-to-end test with a real invoice 🔑

**Secrets required:** `VISION_API_KEY` (Fireworks). This is the first time a real key is needed.

**Files:** none (manual verification). Optionally create a gitignored `.env` for convenience — `.env` is already in `.gitignore`.

- [ ] **Step 1: Confirm the Fireworks model slug**

In the Fireworks console, find the Qwen2.5-VL model id. If it differs from the default in `config.go`, you'll pass it via `VISION_MODEL` in the next step (no code change).

- [ ] **Step 2: Start a local webhook sink to see the payload**

Write a tiny sink that prints any POSTed body and returns 200 (stands in for the teammate's webhook). Create `/tmp/sink.py`:
```python
from http.server import BaseHTTPRequestHandler, HTTPServer

class H(BaseHTTPRequestHandler):
    def do_POST(self):
        n = int(self.headers.get("content-length", 0))
        print(self.rfile.read(n).decode())
        self.send_response(200)
        self.end_headers()

HTTPServer(("", 7000), H).serve_forever()
```
Run it in its own terminal:
```bash
python3 /tmp/sink.py
```

- [ ] **Step 3: Run the scanner pointed at Fireworks + the local sink**

In another terminal (replace the key; set model only if Step 1 said so):
```bash
export VISION_API_KEY="fw_...your_key..."
export WEBHOOK_URL="http://localhost:7000/"
# export VISION_MODEL="accounts/fireworks/models/<confirmed-slug>"   # only if different
go run .
```

- [ ] **Step 4: Scan from your phone or a saved photo**

Open `http://<your-laptop-LAN-ip>:8080/` on your phone (same Wi-Fi) and take a photo of a real Metro/Selgros invoice — or from your laptop:
```bash
curl -s -F "photo=@/path/to/invoice.jpg" localhost:8080/scan
```
Expected: the response HTML lists the invoice line items, and the webhook-sink terminal prints the JSON payload with those items.

- [ ] **Step 5: (Optional) Verify DeepInfra as the fallback**

Re-run Step 3 with:
```bash
export VISION_BASE_URL="https://api.deepinfra.com/v1/openai"
export VISION_MODEL="Qwen/Qwen2.5-VL-72B-Instruct"
export VISION_API_KEY="...deepinfra key..."
```
Expected: same behavior. Confirms the provider seam works. No commit — this task is verification only.

---

## Task 9: Deploy to Hetzner behind Caddy 🔑

**Secrets/access required:** SSH access to the Hetzner VPS; Cloudflare DNS for the domain. *You* run these steps; the plan supplies the commands. No secrets go in the repo.

**Files:**
- Create: `Caddyfile`
- Create: `scanner.service`

- [ ] **Step 1: Add an A record (Cloudflare)**

In Cloudflare DNS, add: `scan.<yourdomain>` → A → `<VPS public IP>`. Set it **DNS only** (grey cloud) so Caddy can complete the ACME HTTP challenge.

- [ ] **Step 2: Build a Linux binary locally**

Build into `bin/` (already gitignored) so the binary is never committed:
```bash
GOOS=linux GOARCH=amd64 go build -o bin/scanner .
```
Expected: `bin/scanner` exists (`file bin/scanner` reports an ELF 64-bit Linux executable).

- [ ] **Step 3: Copy the binary to the VPS (templates are embedded)**

Templates are embedded in the binary via `//go:embed`, so only the binary ships. Create the target dir and copy:
```bash
ssh root@<VPS IP> 'mkdir -p /opt/scanner'
scp bin/scanner root@<VPS IP>:/opt/scanner/scanner
```

- [ ] **Step 4: Create the systemd unit**

Create `scanner.service` (in the repo, for reference) and place its content at `/etc/systemd/system/scanner.service` on the VPS:
```ini
[Unit]
Description=Invoice Scanner
After=network.target

[Service]
WorkingDirectory=/opt/scanner
ExecStart=/opt/scanner/scanner
Environment=PORT=8080
Environment=VISION_API_KEY=fw_...your_key...
Environment=WEBHOOK_URL=http://localhost:<teammate-port>/webhook
Environment=RESTAURANT_ID=demo
Restart=on-failure

[Install]
WantedBy=multi-user.target
```
Then on the VPS:
```bash
systemctl daemon-reload
systemctl enable --now scanner
systemctl status scanner    # expect: active (running)
curl -s localhost:8080/healthz   # expect: ok
```

- [ ] **Step 5: Configure Caddy for automatic HTTPS**

Install Caddy on the VPS (`apt install caddy` or the official repo). Create `Caddyfile` (in the repo for reference) and place it at `/etc/caddy/Caddyfile`:
```
scan.<yourdomain> {
    reverse_proxy localhost:8080
}
```
(Your teammate adds a second block, e.g. `app.<yourdomain> { reverse_proxy localhost:<his-port> }`.)
Then:
```bash
systemctl reload caddy
```

- [ ] **Step 6: Verify end-to-end over HTTPS from a phone**

On your phone, open `https://scan.<yourdomain>/`, photograph a real invoice, and confirm the items render and the webhook receives them.

- [ ] **Step 7: Commit the deploy artifacts**

```bash
git add Caddyfile scanner.service
git commit -m "chore: add Caddy and systemd deploy artifacts"
```

---

## Notes for the implementer

- **Run the whole suite often:** `go test ./...`. Everything except Tasks 8–9 is fully testable offline.
- **The two seams** (`Vision.ExtractItems`, `Webhook.Send`) are the only code that does network I/O. Handlers take them as function values, which is why the handler tests use fakes.
- **Changing the webhook JSON** (when your teammate finalizes it) is a one-file edit in `types.go` plus the `WebhookPayload{...}` construction in `server.go handleScan`.
- **Switching vision provider/model** is env-only: `VISION_BASE_URL`, `VISION_MODEL`, `VISION_API_KEY`.
