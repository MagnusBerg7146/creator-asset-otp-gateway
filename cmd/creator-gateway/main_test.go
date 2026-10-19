package main

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/infrai-examples/creator-asset-otp-gateway/internal/delivery"
)

type fakeCodes struct {
	verified bool
}

func (f fakeCodes) RequestCode(context.Context, string, string) error { return nil }
func (f fakeCodes) VerifyCode(context.Context, string, string, string) (bool, error) {
	return f.verified, nil
}

func TestVerifyCodeControlsDelivery(t *testing.T) {
	tests := []struct {
		name       string
		verified   bool
		wantStatus int
		wantStage  delivery.AssetStage
	}{
		{name: "verified login delivers processed asset", verified: true, wantStatus: http.StatusOK, wantStage: delivery.StageDelivered},
		{name: "unverified login does not deliver", verified: false, wantStatus: http.StatusUnauthorized},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			store := delivery.NewStore(func() time.Time { return time.Unix(100, 0) })
			if err := store.Add(
				delivery.Subscriber{ID: "sub-42", Phone: "+14155550123"},
				delivery.Asset{ID: "asset-2048", SubscriberID: "sub-42", Title: "Creator launch kit", Stage: delivery.StageReady},
			); err != nil {
				t.Fatal(err)
			}
			app := &server{codes: fakeCodes{verified: tt.verified}, store: store}
			req := httptest.NewRequest(http.MethodPost, "/login/verify", bytes.NewBufferString(
				`{"phone":"+14155550123","code":"246810","asset_id":"asset-2048"}`,
			))
			req.Header.Set("Idempotency-Key", "login-2048")
			res := httptest.NewRecorder()
			app.verifyCode(res, req)

			if res.Code != tt.wantStatus {
				t.Fatalf("status = %d, body = %s", res.Code, res.Body.String())
			}
			if tt.wantStatus == http.StatusOK {
				var handoff delivery.Handoff
				if err := json.NewDecoder(res.Body).Decode(&handoff); err != nil {
					t.Fatal(err)
				}
				if handoff.Asset.Stage != tt.wantStage || len(handoff.Subscriber.Updates) != 1 {
					t.Fatalf("handoff = %+v", handoff)
				}
			}
		})
	}
}
