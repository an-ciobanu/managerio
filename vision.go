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
	Model          string     `json:"model"`
	MaxTokens      int        `json:"max_tokens"`
	Messages       []chatMsg  `json:"messages"`
	ResponseFormat respFormat `json:"response_format"`
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
