# Invoice Scanner Client — Design

**Date:** 2026-10-17
**Context:** Hackathon project. Inventory/cost monitoring for small Romanian HoReCa (mom-and-pop restaurants). Owners photograph their Metro/Selgros supplier invoices; the system extracts line items (name, qty, unit, price) and stores them so owners can monitor spending and inventory on a dashboard.

This spec covers **one half** of the system — the client service owned by this author. A teammate separately owns the **webhook + database + dashboard**. The boundary between the two is an HTTP POST of structured JSON.

## Scope

**In scope (this service):**
1. Serve a mobile web page (server-rendered HTML + HTMX) with a camera capture control.
2. Receive an uploaded invoice image.
3. Analyze the image → extract invoice line items as structured data.
4. POST that structured data to the teammate's webhook.
5. Return an HTML fragment showing the result (parsed items, or an error with retry).

**Out of scope (teammate's service):** receiving the webhook, inserting into the database, the monitoring dashboard.

## Non-goals (YAGNI for the hackathon)

- User accounts / authentication (a single `RESTAURANT_ID` env value is enough for the demo).
- Editing/correcting parsed items in the UI before sending.
- Offline support, retries queue, image storage/history.
- Multi-language UI (Romanian-only is fine).

## Priorities

1. **Easy to build** (hackathon timeboxed).
2. **Functional** (works reliably in a live demo on a phone).

## Architecture

A small Go HTTP server (standard library `net/http`, no web framework) that serves SSR HTML with HTMX and runs a capture → analyze → forward pipeline.

### Flow

```
Phone camera
  → <input type="file" accept="image/*" capture="environment">   (opens camera on mobile)
  → HTMX POST (multipart/form-data) to /scan                      (spinner shown via hx-indicator)
  → server reads image bytes → base64 data URI
  → vision model (Qwen2.5-VL, OpenAI-compatible) → JSON items[]
  → map items → webhook payload → POST JSON to teammate's webhook
  → return HTML fragment: parsed items table + "sent ✓"  (or an error fragment with retry)
```

HTMX handles the whole interaction with no custom JavaScript: the form posts the file, the server returns an HTML fragment, HTMX swaps it into the page. The camera is reached with a plain file input (`capture="environment"`), so no `getUserMedia`/JS camera code is needed.

### Routes

| Route | Method | Purpose |
|-------|--------|---------|
| `/` | GET | Upload page: HTML + HTMX + minimal CSS |
| `/scan` | POST | The full pipeline; returns an HTML fragment HTMX swaps in |
| `/healthz` | GET | Trivial health check for the VPS |

### Components (small, focused files)

| File | Responsibility |
|------|----------------|
| `main.go` | Load config from env, wire routes, start server |
| `handlers.go` | `GET /` and `POST /scan` handlers |
| `vision.go` | `ExtractItems(ctx, img []byte) ([]Item, error)` — the vision call; hides the provider entirely |
| `webhook.go` | `SendToWebhook(ctx, payload WebhookPayload) error` |
| `types.go` | `Item`, `WebhookPayload` — the data contract |
| `templates/index.html` | Upload page |
| `templates/fragment.html` | Result/error fragment returned by `/scan` |

Each file has one clear job and can be understood and tested on its own. `ExtractItems` and `SendToWebhook` are the two seams; the handlers orchestrate them.

## Data contract

`types.go` holds the structured shape. This is intentionally the single place to change the JSON — the teammate's webhook contract is still being finalized, so keeping it in one struct makes it trivial to update.

```go
type Item struct {
    Name      string  `json:"name"`
    Qty       float64 `json:"qty"`
    Unit      string  `json:"unit"`       // e.g. "kg", "buc", "l"
    UnitPrice float64 `json:"unit_price"`
}

type WebhookPayload struct {
    RestaurantID string `json:"restaurant_id"`
    Source       string `json:"source"`    // "metro" | "selgros" | ""
    Items        []Item `json:"items"`
}
```

Starting webhook body (easy to change):

```json
{
  "restaurant_id": "demo",
  "source": "metro",
  "items": [
    { "name": "Piept pui", "qty": 5, "unit": "kg", "unit_price": 18.9 }
  ]
}
```

## Analysis engine

**Vision LLM** (not OCR+regex). Metro/Selgros invoices are dense and in Romanian; a vision-language model that returns structured JSON is both easier to build and more robust than classic OCR followed by hand-written parsing.

**Model:** Qwen2.5-VL (open-source), served by an OpenAI-compatible endpoint. It is explicitly designed for structured extraction from scans of invoices/forms/tables, which is exactly this task. Default size: **72B** for accuracy on real photos; drop to 32B/7B if latency or cost requires it (env var only).

**Provider:** provider-agnostic. Both Fireworks and DeepInfra expose an OpenAI-compatible `/chat/completions` endpoint that accepts a base64 image and `response_format` JSON schema. `vision.go` is a single configurable HTTP client:

- Default to **Fireworks** (strongest structured-output / JSON-schema support → most reliable clean JSON).
- **DeepInfra** ready to flip to as a cheaper fallback.
- Switching provider or model is **env-var only**, no code change. (If needed later, the same seam can point at Anthropic instead — but that would use the Anthropic SDK rather than the OpenAI-compatible path.)

**Request shape:** OpenAI-compatible chat completion with a user message whose `content` is `[{type:"text", ...prompt...}, {type:"image_url", image_url:{url:"data:image/jpeg;base64,..."}}]`, plus `response_format` constraining output to the item-list JSON schema. The prompt instructs the model to return each line item with name, quantity, unit, and unit price, in Romanian as printed.

## Configuration (environment variables)

| Var | Purpose | Default |
|-----|---------|---------|
| `PORT` | Listen port | `8080` |
| `VISION_BASE_URL` | OpenAI-compatible base URL (Fireworks or DeepInfra) | Fireworks URL |
| `VISION_API_KEY` | Provider API key | — (required) |
| `VISION_MODEL` | Model slug | Qwen2.5-VL 72B slug |
| `WEBHOOK_URL` | Teammate's webhook endpoint | — (required) |
| `RESTAURANT_ID` | Identifies the restaurant for the DB insert | `demo` |

## Error handling

Every stage fails into a friendly HTML fragment; the server never crashes the request.

- **Bad / missing / oversized image** → "Retake the photo" message. Enforce a max upload size (e.g. 10 MB).
- **Vision call fails or returns no items** → show an error fragment with a retry; **do not** POST to the webhook.
- **Webhook unreachable / non-2xx** → still render the parsed items (so the work isn't lost) plus a note that sending failed and a retry button.
- A visible loading indicator (HTMX `hx-indicator`) covers the few seconds the vision call takes.

## Deployment

- **Host:** Hetzner VPS.
- **Build:** a static Go binary, copied to the VPS and run (systemd unit, or `tmux` for the hackathon).
- **HTTPS:** **Caddy** reverse-proxy in front, with automatic TLS on a subdomain the author controls (A record → VPS IP), proxying to the Go app on `PORT`. Phones need a valid certificate; Caddy's auto-TLS is the lowest-effort way to get one.
- **Fallback (if DNS/cert is slow):** a Cloudflare Tunnel for an instant HTTPS URL, or plain HTTP as a last resort.

### Co-located services

This service and the teammate's dashboard+DB service run as two separate processes on the **same** Hetzner VPS, each on its own port, with Caddy routing to both:

- **Subdomain routing (preferred):** `scan.<domain>` → this app; `app.<domain>` → teammate's dashboard. Each gets its own auto-TLS cert.
- Because both run on the same box, `WEBHOOK_URL` can point at the teammate's service over loopback (e.g. `http://localhost:<his-port>/webhook`) — no public exposure, no CORS, lower latency. It stays an env var, so this is a config choice with no code impact.
- The teammate's dashboard (price/history views reading from the DB) is fully independent of this service.

## Testing (hackathon-lean)

- **Unit:** map a saved sample vision-response JSON → `WebhookPayload` (no network) to lock the data mapping.
- **Unit:** `httptest` mock server for `SendToWebhook` (asserts method, content-type, body shape).
- **Manual end-to-end:** 2–3 real Metro/Selgros invoice photos run through the live flow on a phone.

## Interfaces summary

- `ExtractItems(ctx, img []byte) ([]Item, error)` — takes raw image bytes, returns parsed line items. Depends on `VISION_*` config. Provider/model internal.
- `SendToWebhook(ctx, payload WebhookPayload) error` — posts the payload as JSON. Depends on `WEBHOOK_URL`.
- Handlers depend only on these two functions plus the templates — they contain no provider or HTTP-client detail.
