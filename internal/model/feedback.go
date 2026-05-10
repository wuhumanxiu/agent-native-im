package model

import (
	"time"

	"github.com/uptrace/bun"
)

type FeedbackItem struct {
	bun.BaseModel `bun:"table:feedback_items"`

	ID                int64        `bun:"id,pk,autoincrement" json:"id"`
	PublicID          string       `bun:"public_id,notnull" json:"public_id"`
	SubmitterEntityID int64        `bun:"submitter_entity_id,notnull" json:"submitter_entity_id"`
	Type              string       `bun:"feedback_type,notnull" json:"type"`
	Severity          string       `bun:"severity,notnull" json:"severity"`
	Priority          string       `bun:"priority,notnull" json:"priority"`
	Status            string       `bun:"status,notnull" json:"status"`
	Title             string       `bun:"title,notnull" json:"title"`
	Description       string       `bun:"description,notnull" json:"description"`
	Contact           string       `bun:"contact,notnull" json:"contact,omitempty"`
	Attachments       []Attachment `bun:"attachments,type:jsonb,notnull,default:'[]'" json:"attachments,omitempty"`
	CreatedAt         time.Time    `bun:"created_at,nullzero,notnull,default:now()" json:"created_at"`
	UpdatedAt         time.Time    `bun:"updated_at,nullzero,notnull,default:now()" json:"updated_at"`
	LastCommentAt     *time.Time   `bun:"last_comment_at" json:"last_comment_at,omitempty"`

	Submitter *Entity `bun:"rel:belongs-to,join:submitter_entity_id=id" json:"submitter,omitempty"`
}

type FeedbackComment struct {
	bun.BaseModel `bun:"table:feedback_comments"`

	ID             int64        `bun:"id,pk,autoincrement" json:"id"`
	FeedbackID     int64        `bun:"feedback_id,notnull" json:"feedback_id"`
	AuthorEntityID int64        `bun:"author_entity_id,notnull" json:"author_entity_id"`
	Body           string       `bun:"body,notnull" json:"body"`
	Visibility     string       `bun:"visibility,notnull" json:"visibility"`
	Attachments    []Attachment `bun:"attachments,type:jsonb,notnull,default:'[]'" json:"attachments,omitempty"`
	CreatedAt      time.Time    `bun:"created_at,nullzero,notnull,default:now()" json:"created_at"`

	Author *Entity `bun:"rel:belongs-to,join:author_entity_id=id" json:"author,omitempty"`
}

type FeedbackListFilter struct {
	SubmitterEntityID *int64
	Status            string
	Type              string
	Query             string
	Limit             int
	Offset            int
}
