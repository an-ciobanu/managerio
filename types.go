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
