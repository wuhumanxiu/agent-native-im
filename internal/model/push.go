package model

import (
	"time"

	"github.com/uptrace/bun"
)

type PushSubscription struct {
	bun.BaseModel `bun:"table:push_subscriptions"`

	ID            int64     `bun:"id,pk,autoincrement" json:"id"`
	EntityID      int64     `bun:"entity_id,notnull" json:"entity_id"`
	Provider      string    `bun:"provider,notnull,default:'webpush'" json:"provider"`
	Platform      string    `bun:"platform,notnull,default:''" json:"platform"`
	DeviceID      string    `bun:"device_id" json:"device_id"`
	Endpoint      string    `bun:"endpoint,notnull" json:"endpoint"`
	KeyP256DH     string    `bun:"key_p256dh,notnull" json:"-"`
	KeyAuth       string    `bun:"key_auth,notnull" json:"-"`
	LastSuccessAt time.Time `bun:"last_success_at,nullzero" json:"last_success_at,omitempty"`
	LastErrorAt   time.Time `bun:"last_error_at,nullzero" json:"last_error_at,omitempty"`
	LastError     string    `bun:"last_error,notnull,default:''" json:"last_error,omitempty"`
	CreatedAt     time.Time `bun:"created_at,nullzero,notnull,default:now()" json:"created_at"`
}

const (
	PushProviderWebPush = "webpush"
	PushProviderExpo    = "expo"
)
