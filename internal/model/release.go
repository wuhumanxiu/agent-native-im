package model

import (
	"time"

	"github.com/uptrace/bun"
)

type ReleaseSection struct {
	Kind  string   `json:"kind"`
	Title string   `json:"title"`
	Items []string `json:"items"`
}

type ReleaseAction struct {
	Component string `json:"component"`
	Title     string `json:"title"`
	Body      string `json:"body"`
	URL       string `json:"url,omitempty"`
}

type Release struct {
	bun.BaseModel `bun:"table:releases"`

	ID              int64            `bun:"id,pk,autoincrement" json:"id"`
	PublicID        string           `bun:"public_id,notnull" json:"public_id"`
	Version         string           `bun:"version,notnull" json:"version"`
	Component       string           `bun:"component,notnull" json:"component"`
	Platform        string           `bun:"platform,notnull" json:"platform"`
	Channel         string           `bun:"channel,notnull" json:"channel"`
	Title           string           `bun:"title,notnull" json:"title"`
	Summary         string           `bun:"summary,notnull" json:"summary"`
	Sections        []ReleaseSection `bun:"sections,type:jsonb,notnull,default:'[]'" json:"sections"`
	RequiredActions []ReleaseAction  `bun:"required_actions,type:jsonb,notnull,default:'[]'" json:"required_actions"`
	KnownIssues     []string         `bun:"known_issues,type:jsonb,notnull,default:'[]'" json:"known_issues"`
	PublishedAt     time.Time        `bun:"published_at,nullzero,notnull,default:now()" json:"published_at"`
	CreatedAt       time.Time        `bun:"created_at,nullzero,notnull,default:now()" json:"created_at"`

	ReadAt *time.Time `bun:"-" json:"read_at,omitempty"`
	IsRead bool       `bun:"-" json:"is_read"`
}

type ReleaseRead struct {
	bun.BaseModel `bun:"table:release_reads"`

	EntityID  int64     `bun:"entity_id,pk" json:"entity_id"`
	ReleaseID int64     `bun:"release_id,pk" json:"release_id"`
	ReadAt    time.Time `bun:"read_at,nullzero,notnull,default:now()" json:"read_at"`
}

type FeedbackReleaseLink struct {
	bun.BaseModel `bun:"table:feedback_release_links"`

	FeedbackID int64     `bun:"feedback_id,pk" json:"feedback_id"`
	ReleaseID  int64     `bun:"release_id,pk" json:"release_id"`
	LinkType   string    `bun:"link_type,pk" json:"link_type"`
	CreatedAt  time.Time `bun:"created_at,nullzero,notnull,default:now()" json:"created_at"`

	Release *Release `bun:"rel:belongs-to,join:release_id=id" json:"release,omitempty"`
}

type ReleaseListFilter struct {
	Component string
	Platform  string
	Channel   string
	Limit     int
	Offset    int
}
