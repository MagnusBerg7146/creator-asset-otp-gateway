package main

import (
	"context"
	"encoding/json"
	"errors"
	"log"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/infrai-examples/creator-asset-otp-gateway/internal/delivery"
	"github.com/infrai-examples/creator-asset-otp-gateway/internal/infrai"
)

type codeVerifier interface {
	RequestCode(context.Context, string, string) error
	VerifyCode(context.Context, string, string, string) (bool, error)
}

type server struct {
	codes codeVerifier
	store *delivery.Store
}

type loginRequest struct {
	Phone   string `json:"phone"`
	Code    string `json:"code,omitempty"`
	AssetID string `json:"asset_id,omitempty"`
}

func main() {
	key := os.Getenv("INFRAI_API_KEY")
	if key == "" {
		log.Fatal("INFRAI_API_KEY is required")
	}
	addr := os.Getenv("ADDR")
	if addr == "" {
		addr = ":8080"
	}

	store := delivery.NewStore(time.Now)
	if err := store.Add(
		delivery.Subscriber{ID: "sub-42", Phone: "+14155550123", Updates: []string{"release:published"}},
		delivery.Asset{ID: "asset-2048", SubscriberID: "sub-42", Title: "Creator launch kit", Stage: delivery.StageReady},
	); err != nil {
		log.Fatal(err)
	}
	app := &server{codes: infrai.NewOTPClient(key), store: store}
	mux := http.NewServeMux()
	mux.HandleFunc("POST /login/code", app.requestCode)
	mux.HandleFunc("POST /login/verify", app.verifyCode)
	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, _ *http.Request) {
		writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
	})

	log.Printf("creator gateway listening on %s", addr)
	log.Fatal(http.ListenAndServe(addr, mux))
}

func (s *server) requestCode(w http.ResponseWriter, r *http.Request) {
	var input loginRequest
	operationID := strings.TrimSpace(r.Header.Get("Idempotency-Key"))
	if decodeJSON(w, r, &input) != nil {
		return
	}
	if input.Phone == "" || operationID == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "phone and Idempotency-Key are required"})
		return
	}
	if err := s.codes.RequestCode(r.Context(), input.Phone, operationID+":send"); err != nil {
		writeServiceError(w, err)
		return
	}
	writeJSON(w, http.StatusAccepted, map[string]string{"status": "code_sent"})
}

func (s *server) verifyCode(w http.ResponseWriter, r *http.Request) {
	var input loginRequest
	operationID := strings.TrimSpace(r.Header.Get("Idempotency-Key"))
	if decodeJSON(w, r, &input) != nil {
		return
	}
	if input.Phone == "" || input.Code == "" || input.AssetID == "" || operationID == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "phone, code, asset_id, and Idempotency-Key are required"})
		return
	}
	verified, err := s.codes.VerifyCode(r.Context(), input.Phone, input.Code, operationID+":verify")
	if err != nil {
		writeServiceError(w, err)
		return
	}
	handoff, err := s.store.DeliverVerified(input.Phone, input.AssetID, verified)
	if err != nil {
		writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "login not verified"})
		return
	}
	writeJSON(w, http.StatusOK, handoff)
}

func writeServiceError(w http.ResponseWriter, err error) {
	var apiErr *infrai.APIError
	if errors.As(err, &apiErr) && apiErr.HTTPStatus >= 400 && apiErr.HTTPStatus < 500 {
		writeJSON(w, apiErr.HTTPStatus, map[string]string{"error": apiErr.Code})
		return
	}
	writeJSON(w, http.StatusBadGateway, map[string]string{"error": "sms request failed"})
}

func decodeJSON(w http.ResponseWriter, r *http.Request, target any) error {
	decoder := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<20))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(target); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid JSON body"})
		return err
	}
	return nil
}

func writeJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(value); err != nil {
		log.Printf("encode response: %v", err)
	}
}
