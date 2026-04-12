package postgres

import (
	"context"
	"time"

	"github.com/wzfukui/agent-native-im/internal/model"
)

func (s *PGStore) CreatePushSubscription(ctx context.Context, sub *model.PushSubscription) error {
	if sub.Provider == "" {
		sub.Provider = model.PushProviderWebPush
	}
	_, err := s.DB.NewInsert().Model(sub).
		On("CONFLICT (entity_id, provider, endpoint) DO UPDATE").
		Set("provider = EXCLUDED.provider").
		Set("platform = EXCLUDED.platform").
		Set("key_p256dh = EXCLUDED.key_p256dh").
		Set("key_auth = EXCLUDED.key_auth").
		Set("device_id = EXCLUDED.device_id").
		Set("last_error = ''").
		Set("last_error_at = NULL").
		Exec(ctx)
	return err
}

func (s *PGStore) DeletePushSubscription(ctx context.Context, entityID int64, endpoint string) error {
	_, err := s.DB.NewDelete().Model((*model.PushSubscription)(nil)).
		Where("entity_id = ?", entityID).
		Where("endpoint = ?", endpoint).
		Exec(ctx)
	return err
}

func (s *PGStore) GetPushSubscriptionsByEntity(ctx context.Context, entityID int64) ([]*model.PushSubscription, error) {
	var subs []*model.PushSubscription
	err := s.DB.NewSelect().Model(&subs).
		Where("entity_id = ?", entityID).
		Scan(ctx)
	return subs, err
}

func (s *PGStore) UpdatePushSubscriptionDeliveryStatus(ctx context.Context, id int64, lastError string, success bool) error {
	update := s.DB.NewUpdate().Model((*model.PushSubscription)(nil)).Where("id = ?", id)
	now := time.Now()
	if success {
		_, err := update.
			Set("last_success_at = ?", now).
			Set("last_error_at = NULL").
			Set("last_error = ''").
			Exec(ctx)
		return err
	}
	_, err := update.
		Set("last_error_at = ?", now).
		Set("last_error = ?", lastError).
		Exec(ctx)
	return err
}

func (s *PGStore) DeleteAllPushSubscriptions(ctx context.Context) error {
	_, err := s.DB.NewDelete().Model((*model.PushSubscription)(nil)).Exec(ctx)
	return err
}
