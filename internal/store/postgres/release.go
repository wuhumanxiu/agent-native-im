package postgres

import (
	"context"
	"encoding/json"
	"strings"
	"time"

	"github.com/uptrace/bun"
	"github.com/wzfukui/agent-native-im/internal/model"
)

func (s *PGStore) CreateRelease(ctx context.Context, release *model.Release) error {
	if release != nil {
		if release.TitleI18N == nil {
			release.TitleI18N = map[string]string{}
		}
		if release.SummaryI18N == nil {
			release.SummaryI18N = map[string]string{}
		}
		if release.SectionsI18N == nil {
			release.SectionsI18N = map[string][]model.ReleaseSection{}
		}
		if release.ActionsI18N == nil {
			release.ActionsI18N = map[string][]model.ReleaseAction{}
		}
		if release.KnownIssuesI18N == nil {
			release.KnownIssuesI18N = map[string][]string{}
		}
	}
	_, err := s.DB.NewInsert().Model(release).Exec(ctx)
	if err != nil {
		return err
	}
	return s.createReleaseNotifications(ctx, release)
}

func (s *PGStore) GetReleaseByID(ctx context.Context, id int64) (*model.Release, error) {
	release := new(model.Release)
	err := s.DB.NewSelect().Model(release).Where("release.id = ?", id).Scan(ctx)
	if err != nil {
		return nil, err
	}
	return release, nil
}

func (s *PGStore) ListReleases(ctx context.Context, filter model.ReleaseListFilter, readerEntityID int64) ([]*model.Release, int, error) {
	if filter.Limit <= 0 || filter.Limit > 100 {
		filter.Limit = 30
	}
	if filter.Offset < 0 {
		filter.Offset = 0
	}
	channel := strings.TrimSpace(filter.Channel)
	if channel == "" {
		channel = "production"
	}

	var releases []*model.Release
	q := s.DB.NewSelect().
		Model(&releases).
		Where("release.channel = ?", channel).
		OrderExpr("release.published_at DESC, release.id DESC").
		Limit(filter.Limit).
		Offset(filter.Offset)
	if component := strings.TrimSpace(filter.Component); component != "" {
		q = q.Where("release.component = ?", component)
	}
	if platform := strings.TrimSpace(filter.Platform); platform != "" {
		q = q.Where("release.platform = ? OR release.platform = 'all'", platform)
	}
	total, err := q.ScanAndCount(ctx)
	if err != nil {
		return nil, 0, err
	}
	if readerEntityID > 0 && len(releases) > 0 {
		if err := s.attachReleaseReads(ctx, releases, readerEntityID); err != nil {
			return nil, 0, err
		}
	}
	return releases, total, nil
}

func (s *PGStore) GetLatestRelease(ctx context.Context, channel string, readerEntityID int64) (*model.Release, error) {
	if strings.TrimSpace(channel) == "" {
		channel = "production"
	}
	release := new(model.Release)
	err := s.DB.NewSelect().
		Model(release).
		Where("release.channel = ?", channel).
		OrderExpr("release.published_at DESC, release.id DESC").
		Limit(1).
		Scan(ctx)
	if err != nil {
		return nil, err
	}
	if readerEntityID > 0 {
		if err := s.attachReleaseReads(ctx, []*model.Release{release}, readerEntityID); err != nil {
			return nil, err
		}
	}
	return release, nil
}

func (s *PGStore) MarkReleaseRead(ctx context.Context, entityID, releaseID int64) error {
	_, err := s.DB.NewInsert().
		Model(&model.ReleaseRead{EntityID: entityID, ReleaseID: releaseID}).
		On("CONFLICT (entity_id, release_id) DO UPDATE").
		Set("read_at = now()").
		Exec(ctx)
	return err
}

func (s *PGStore) CountUnreadReleases(ctx context.Context, entityID int64, channel string) (int, error) {
	if strings.TrimSpace(channel) == "" {
		channel = "production"
	}
	return s.DB.NewSelect().
		Model((*model.Release)(nil)).
		Where("release.channel = ?", channel).
		Where("NOT EXISTS (?)",
			s.DB.NewSelect().
				Model((*model.ReleaseRead)(nil)).
				ColumnExpr("1").
				Where("release_read.release_id = release.id").
				Where("release_read.entity_id = ?", entityID),
		).
		Count(ctx)
}

func (s *PGStore) createReleaseNotifications(ctx context.Context, release *model.Release) error {
	if release == nil || release.ID <= 0 || release.Channel != "production" {
		return nil
	}
	titleI18N, err := json.Marshal(release.TitleI18N)
	if err != nil {
		return err
	}
	bodyI18N, err := json.Marshal(release.SummaryI18N)
	if err != nil {
		return err
	}
	now := time.Now()
	_, err = s.DB.NewRaw(`
		INSERT INTO notifications (
			recipient_entity_id,
			kind,
			status,
			title,
			body,
			data,
			created_at,
			updated_at
		)
		SELECT
			entity.id,
			?,
			?,
			?,
			?,
			jsonb_build_object(
				'release_id', ?::bigint,
				'release_public_id', ?::text,
				'version', ?::text,
				'path', ?::text,
				'title_i18n', ?::jsonb,
				'body_i18n', ?::jsonb
			),
			?,
			?
		FROM entities AS entity
		WHERE entity.entity_type = ? AND entity.status = ?
		ON CONFLICT DO NOTHING
	`,
		"release.published",
		model.NotificationUnread,
		"ANI update "+release.Version,
		release.Title,
		release.ID,
		release.PublicID,
		release.Version,
		"/settings/releases",
		string(titleI18N),
		string(bodyI18N),
		now,
		now,
		model.EntityUser,
		"active",
	).Exec(ctx)
	return err
}

func (s *PGStore) attachReleaseReads(ctx context.Context, releases []*model.Release, entityID int64) error {
	ids := make([]int64, 0, len(releases))
	byID := make(map[int64]*model.Release, len(releases))
	for _, release := range releases {
		ids = append(ids, release.ID)
		byID[release.ID] = release
	}
	var reads []*model.ReleaseRead
	if err := s.DB.NewSelect().
		Model(&reads).
		Where("release_read.entity_id = ?", entityID).
		Where("release_read.release_id IN (?)", bun.In(ids)).
		Scan(ctx); err != nil {
		return err
	}
	for _, read := range reads {
		if release := byID[read.ReleaseID]; release != nil {
			readAt := read.ReadAt
			release.ReadAt = &readAt
			release.IsRead = true
		}
	}
	return nil
}
