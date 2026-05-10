package postgres

import (
	"context"
	"strings"
	"time"

	"github.com/uptrace/bun"
	"github.com/wzfukui/agent-native-im/internal/model"
)

func (s *PGStore) CreateFeedbackItem(ctx context.Context, item *model.FeedbackItem) error {
	_, err := s.DB.NewInsert().Model(item).Exec(ctx)
	return err
}

func (s *PGStore) GetFeedbackItemByID(ctx context.Context, id int64) (*model.FeedbackItem, error) {
	item := new(model.FeedbackItem)
	err := s.DB.NewSelect().
		Model(item).
		Relation("Submitter").
		Where("feedback_item.id = ?", id).
		Scan(ctx)
	if err != nil {
		return nil, err
	}
	return item, nil
}

func (s *PGStore) ListFeedbackItems(ctx context.Context, filter model.FeedbackListFilter) ([]*model.FeedbackItem, int, error) {
	if filter.Limit <= 0 || filter.Limit > 200 {
		filter.Limit = 50
	}
	if filter.Offset < 0 {
		filter.Offset = 0
	}

	var items []*model.FeedbackItem
	q := s.DB.NewSelect().
		Model(&items).
		Relation("Submitter").
		OrderExpr("feedback_item.updated_at DESC").
		Limit(filter.Limit).
		Offset(filter.Offset)
	if filter.SubmitterEntityID != nil {
		q = q.Where("feedback_item.submitter_entity_id = ?", *filter.SubmitterEntityID)
	}
	if strings.TrimSpace(filter.Status) != "" {
		q = q.Where("feedback_item.status = ?", strings.TrimSpace(filter.Status))
	}
	if strings.TrimSpace(filter.Type) != "" {
		q = q.Where("feedback_item.feedback_type = ?", strings.TrimSpace(filter.Type))
	}
	if query := strings.TrimSpace(filter.Query); query != "" {
		like := "%" + query + "%"
		q = q.WhereGroup(" AND ", func(q *bun.SelectQuery) *bun.SelectQuery {
			return q.Where("feedback_item.title ILIKE ?", like).WhereOr("feedback_item.description ILIKE ?", like)
		})
	}
	total, err := q.ScanAndCount(ctx)
	return items, total, err
}

func (s *PGStore) UpdateFeedbackItem(ctx context.Context, item *model.FeedbackItem) error {
	item.UpdatedAt = time.Now()
	_, err := s.DB.NewUpdate().Model(item).WherePK().Exec(ctx)
	return err
}

func (s *PGStore) CreateFeedbackComment(ctx context.Context, comment *model.FeedbackComment) error {
	_, err := s.DB.NewInsert().Model(comment).Exec(ctx)
	if err != nil {
		return err
	}
	now := time.Now()
	_, err = s.DB.NewUpdate().
		Model((*model.FeedbackItem)(nil)).
		Set("last_comment_at = ?", now).
		Set("updated_at = ?", now).
		Where("id = ?", comment.FeedbackID).
		Exec(ctx)
	return err
}

func (s *PGStore) ListFeedbackComments(ctx context.Context, feedbackID int64, includeInternal bool) ([]*model.FeedbackComment, error) {
	var comments []*model.FeedbackComment
	q := s.DB.NewSelect().
		Model(&comments).
		Relation("Author").
		Where("feedback_comment.feedback_id = ?", feedbackID).
		OrderExpr("feedback_comment.created_at ASC")
	if !includeInternal {
		q = q.Where("feedback_comment.visibility = ?", "public")
	}
	err := q.Scan(ctx)
	return comments, err
}
