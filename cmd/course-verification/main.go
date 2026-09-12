package main

import (
	"encoding/json"
	"errors"
	"log"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"

	verification "example.com/edtech-verification"
)

type server struct {
	workflow verification.SignupWorkflow
	mu       sync.RWMutex
	entries  map[string]verification.Enrollment
}

func main() {
	apiKey := os.Getenv("INFRAI_API_KEY")
	if apiKey == "" {
		log.Fatal("INFRAI_API_KEY is required")
	}
	s := &server{
		workflow: verification.SignupWorkflow{Email: &verification.InfraiEmailClient{APIKey: apiKey}},
		entries:  make(map[string]verification.Enrollment),
	}
	mux := http.NewServeMux()
	mux.HandleFunc("POST /signups", s.startSignup)
	mux.HandleFunc("GET /educator/reports/{message_id}", s.report)
	address := os.Getenv("LISTEN_ADDR")
	if address == "" {
		address = ":8080"
	}
	httpServer := &http.Server{Addr: address, Handler: mux, ReadHeaderTimeout: 5 * time.Second}
	log.Printf("course verification service listening on %s", address)
	log.Fatal(httpServer.ListenAndServe())
}

func (s *server) startSignup(w http.ResponseWriter, r *http.Request) {
	var input verification.Signup
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 64<<10)).Decode(&input); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid signup body"})
		return
	}
	enrollment, err := s.workflow.Start(r.Context(), input)
	if err != nil {
		writeWorkflowError(w, err)
		return
	}
	s.mu.Lock()
	s.entries[enrollment.MessageID] = enrollment
	s.mu.Unlock()
	writeJSON(w, http.StatusAccepted, enrollment)
}

func (s *server) report(w http.ResponseWriter, r *http.Request) {
	messageID := strings.TrimSpace(r.PathValue("message_id"))
	s.mu.RLock()
	enrollment, found := s.entries[messageID]
	s.mu.RUnlock()
	if !found {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "enrollment not found"})
		return
	}
	report, err := s.workflow.Report(r.Context(), enrollment)
	if err != nil {
		writeWorkflowError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, report)
}

func writeWorkflowError(w http.ResponseWriter, err error) {
	var apiErr *verification.APIError
	if errors.As(err, &apiErr) && apiErr.HTTPStatus >= 400 && apiErr.HTTPStatus < 500 {
		writeJSON(w, apiErr.HTTPStatus, map[string]string{"error": apiErr.Message})
		return
	}
	writeJSON(w, http.StatusBadGateway, map[string]string{"error": "email delivery request failed"})
}

func writeJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(value); err != nil {
		log.Printf("encode response: %v", err)
	}
}
