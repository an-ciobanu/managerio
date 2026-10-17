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
