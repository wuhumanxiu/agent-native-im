package mention

import (
	"context"
	"errors"
	"strings"

	"github.com/wzfukui/agent-native-im/internal/model"
)

var ErrMentionNotParticipant = errors.New("mentioned entity is not a participant")

type ResolverStore interface {
	GetEntityByPublicID(ctx context.Context, publicID string) (*model.Entity, error)
	GetEntitiesByIDs(ctx context.Context, ids []int64) ([]*model.Entity, error)
	IsParticipant(ctx context.Context, conversationID, entityID int64) (bool, error)
}

// ResolveEntityIDs converts the public mention protocol into the internal IDs
// that the existing database and router use. Legacy numeric mentions remain
// supported while clients migrate to UUID public IDs.
func ResolveEntityIDs(
	ctx context.Context,
	store ResolverStore,
	conversationID int64,
	legacyIDs []int64,
	publicIDs []string,
) ([]int64, error) {
	resolved := make([]int64, 0, len(legacyIDs)+len(publicIDs))
	seen := map[int64]struct{}{}

	add := func(entityID int64) error {
		if entityID <= 0 {
			return ErrMentionNotParticipant
		}
		isMember, err := store.IsParticipant(ctx, conversationID, entityID)
		if err != nil || !isMember {
			return ErrMentionNotParticipant
		}
		if _, ok := seen[entityID]; ok {
			return nil
		}
		seen[entityID] = struct{}{}
		resolved = append(resolved, entityID)
		return nil
	}

	for _, entityID := range legacyIDs {
		if err := add(entityID); err != nil {
			return nil, err
		}
	}

	for _, rawPublicID := range publicIDs {
		publicID := strings.TrimSpace(rawPublicID)
		if publicID == "" {
			continue
		}
		entity, err := store.GetEntityByPublicID(ctx, publicID)
		if err != nil || entity == nil {
			return nil, ErrMentionNotParticipant
		}
		if err := add(entity.ID); err != nil {
			return nil, err
		}
	}

	return resolved, nil
}

func PublicIDsForEntityIDs(ctx context.Context, store ResolverStore, entityIDs []int64) []string {
	if len(entityIDs) == 0 {
		return nil
	}
	entities, err := store.GetEntitiesByIDs(ctx, entityIDs)
	if err != nil {
		return nil
	}
	byID := make(map[int64]string, len(entities))
	for _, entity := range entities {
		if entity == nil || strings.TrimSpace(entity.PublicID) == "" {
			continue
		}
		byID[entity.ID] = entity.PublicID
	}
	publicIDs := make([]string, 0, len(entityIDs))
	for _, entityID := range entityIDs {
		if publicID := byID[entityID]; publicID != "" {
			publicIDs = append(publicIDs, publicID)
		}
	}
	return publicIDs
}

func MentionedEntities(ctx context.Context, store ResolverStore, entityIDs []int64) []*model.Entity {
	if len(entityIDs) == 0 {
		return nil
	}
	entities, err := store.GetEntitiesByIDs(ctx, entityIDs)
	if err != nil {
		return nil
	}
	byID := make(map[int64]*model.Entity, len(entities))
	for _, entity := range entities {
		if entity != nil {
			byID[entity.ID] = entity
		}
	}
	ordered := make([]*model.Entity, 0, len(entityIDs))
	seen := map[int64]struct{}{}
	for _, entityID := range entityIDs {
		entity := byID[entityID]
		if entity == nil {
			continue
		}
		if _, ok := seen[entityID]; ok {
			continue
		}
		seen[entityID] = struct{}{}
		ordered = append(ordered, entity)
	}
	return ordered
}
