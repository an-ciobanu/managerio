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
