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
