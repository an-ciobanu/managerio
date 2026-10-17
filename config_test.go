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
