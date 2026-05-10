package postgres

import (
	"context"
	"time"

	"github.com/wzfukui/agent-native-im/internal/model"
)

func (s *PGStore) CreateExternalIdentity(ctx context.Context, identity *model.ExternalIdentity) error {
	_, err := s.DB.NewInsert().Model(identity).Exec(ctx)
	return err
}

func (s *PGStore) GetExternalIdentityByID(ctx context.Context, id int64) (*model.ExternalIdentity, error) {
	identity := new(model.ExternalIdentity)
	err := s.DB.NewSelect().Model(identity).Where("id = ?", id).Scan(ctx)
	if err != nil {
		return nil, err
	}
	return identity, nil
}

func (s *PGStore) GetExternalIdentityByProviderSubject(ctx context.Context, provider, providerSubject string) (*model.ExternalIdentity, error) {
	identity := new(model.ExternalIdentity)
	err := s.DB.NewSelect().Model(identity).
		Where("provider = ?", provider).
		Where("provider_subject = ?", providerSubject).
		Scan(ctx)
	if err != nil {
		return nil, err
	}
	return identity, nil
}

func (s *PGStore) ListExternalIdentitiesByEntity(ctx context.Context, entityID int64) ([]*model.ExternalIdentity, error) {
	var identities []*model.ExternalIdentity
	err := s.DB.NewSelect().Model(&identities).
		Where("entity_id = ?", entityID).
		OrderExpr("linked_at ASC").
		Scan(ctx)
	return identities, err
}

func (s *PGStore) UpdateExternalIdentity(ctx context.Context, identity *model.ExternalIdentity) error {
	if identity.LastUsedAt.IsZero() {
		identity.LastUsedAt = time.Now()
	}
	_, err := s.DB.NewUpdate().Model(identity).WherePK().Exec(ctx)
	return err
}

func (s *PGStore) DeleteExternalIdentity(ctx context.Context, identityID int64) error {
	_, err := s.DB.NewDelete().Model((*model.ExternalIdentity)(nil)).
		Where("id = ?", identityID).
		Exec(ctx)
	return err
}
