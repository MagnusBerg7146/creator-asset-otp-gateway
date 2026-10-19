package delivery

import (
	"errors"
	"sync"
	"time"
)

type AssetStage string

const (
	StageUploaded   AssetStage = "uploaded"
	StageProcessing AssetStage = "processing"
	StageReady      AssetStage = "ready"
	StageDelivered  AssetStage = "delivered"
)

type Subscriber struct {
	ID          string    `json:"id"`
	Phone       string    `json:"phone"`
	Updates     []string  `json:"updates"`
	LastLoginAt time.Time `json:"last_login_at"`
}

type Asset struct {
	ID           string     `json:"id"`
	SubscriberID string     `json:"subscriber_id"`
	Title        string     `json:"title"`
	Stage        AssetStage `json:"stage"`
}

type Handoff struct {
	Subscriber Subscriber `json:"subscriber"`
	Asset      Asset      `json:"asset"`
}

type Store struct {
	mu          sync.Mutex
	subscribers map[string]Subscriber
	assets      map[string]Asset
	now         func() time.Time
}

func NewStore(now func() time.Time) *Store {
	return &Store{
		subscribers: make(map[string]Subscriber),
		assets:      make(map[string]Asset),
		now:         now,
	}
}

func (s *Store) Add(subscriber Subscriber, asset Asset) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if subscriber.ID == "" || subscriber.Phone == "" || asset.ID == "" || asset.SubscriberID != subscriber.ID {
		return errors.New("subscriber and owned asset are required")
	}
	if asset.Stage != StageReady {
		return errors.New("asset must finish processing before login delivery")
	}
	s.subscribers[subscriber.Phone] = subscriber
	s.assets[asset.ID] = asset
	return nil
}

func (s *Store) DeliverVerified(phone, assetID string, verified bool) (Handoff, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if !verified {
		return Handoff{}, errors.New("phone code was not verified")
	}
	subscriber, ok := s.subscribers[phone]
	if !ok {
		return Handoff{}, errors.New("subscriber not found")
	}
	asset, ok := s.assets[assetID]
	if !ok || asset.SubscriberID != subscriber.ID {
		return Handoff{}, errors.New("asset not found for subscriber")
	}
	if asset.Stage != StageReady && asset.Stage != StageDelivered {
		return Handoff{}, errors.New("asset is not ready")
	}
	asset.Stage = StageDelivered
	subscriber.LastLoginAt = s.now().UTC()
	subscriber.Updates = appendUnique(subscriber.Updates, "asset:"+asset.ID+":delivered")
	s.subscribers[phone] = subscriber
	s.assets[asset.ID] = asset
	return Handoff{Subscriber: subscriber, Asset: asset}, nil
}

func appendUnique(values []string, value string) []string {
	for _, current := range values {
		if current == value {
			return values
		}
	}
	return append(values, value)
}
