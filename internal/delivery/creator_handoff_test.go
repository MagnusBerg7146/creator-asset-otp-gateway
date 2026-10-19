package delivery

import (
	"testing"
	"time"
)

func TestDeliverVerified(t *testing.T) {
	fixed := time.Date(2026, 9, 6, 10, 0, 0, 0, time.UTC)
	tests := []struct {
		name      string
		verified  bool
		stage     AssetStage
		wantStage AssetStage
		wantError bool
	}{
		{name: "verified code releases ready asset", verified: true, stage: StageReady, wantStage: StageDelivered},
		{name: "rejected code releases nothing", verified: false, stage: StageReady, wantError: true},
		{name: "unfinished processing stays private", verified: true, stage: StageProcessing, wantError: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			store := NewStore(func() time.Time { return fixed })
			subscriber := Subscriber{ID: "sub-42", Phone: "+14155550123"}
			asset := Asset{ID: "asset-2048", SubscriberID: subscriber.ID, Title: "Creator launch kit", Stage: StageReady}
			if tt.stage == StageReady {
				if err := store.Add(subscriber, asset); err != nil {
					t.Fatal(err)
				}
			} else {
				store.subscribers[subscriber.Phone] = subscriber
				asset.Stage = tt.stage
				store.assets[asset.ID] = asset
			}

			got, err := store.DeliverVerified(subscriber.Phone, asset.ID, tt.verified)
			if (err != nil) != tt.wantError {
				t.Fatalf("DeliverVerified() error = %v, wantError %v", err, tt.wantError)
			}
			if tt.wantError {
				if store.assets[asset.ID].Stage != tt.stage {
					t.Fatalf("asset stage changed after rejected handoff")
				}
				return
			}
			if got.Asset.Stage != tt.wantStage || !got.Subscriber.LastLoginAt.Equal(fixed) {
				t.Fatalf("handoff = %+v", got)
			}
			if len(got.Subscriber.Updates) != 1 || got.Subscriber.Updates[0] != "asset:asset-2048:delivered" {
				t.Fatalf("updates = %v", got.Subscriber.Updates)
			}
		})
	}
}
