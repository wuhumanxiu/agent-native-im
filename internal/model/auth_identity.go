package model

import (
	"encoding/json"
	"time"

	"github.com/uptrace/bun"
)

type ExternalIdentity struct {
	bun.BaseModel `bun:"table:auth_external_identities"`

	ID               int64           `bun:"id,pk,autoincrement" json:"id"`
	EntityID         int64           `bun:"entity_id,notnull" json:"entity_id"`
	Provider         string          `bun:"provider,notnull" json:"provider"`
	ProviderSubject  string          `bun:"provider_subject,notnull" json:"-"`
	UpstreamProvider string          `bun:"upstream_provider,notnull" json:"upstream_provider"`
	UpstreamSubject  string          `bun:"upstream_subject,notnull" json:"-"`
	SiteID           string          `bun:"site_id,notnull" json:"-"`
	DisplayName      string          `bun:"display_name,notnull" json:"display_name,omitempty"`
	AvatarURL        string          `bun:"avatar_url,notnull" json:"avatar_url,omitempty"`
	RawProfile       json.RawMessage `bun:"raw_profile,type:jsonb,notnull,default:'{}'" json:"-"`
	LinkedAt         time.Time       `bun:"linked_at,nullzero,notnull,default:now()" json:"linked_at"`
	LastUsedAt       time.Time       `bun:"last_used_at,nullzero,notnull,default:now()" json:"last_used_at"`
}
