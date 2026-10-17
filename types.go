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
//
// Input distinguishes the two scan directions for the inventory math:
//   - true:  a supplier invoice (factură, e.g. Metro/Selgros) — goods IN (adds stock)
//   - false: a client fiscal receipt (bon fiscal) — goods OUT (removes stock)
type WebhookPayload struct {
	RestaurantID string `json:"restaurant_id"`
	Source       string `json:"source"`
	Input        bool   `json:"input"`
	Items        []Item `json:"items"`
}

// resultView is the data passed to templates/fragment.html.
type resultView struct {
	Items      []Item
	Input      bool
	SendFailed bool
}
