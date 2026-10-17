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
