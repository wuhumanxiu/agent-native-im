package mention

import (
	"context"
	"errors"
	"strings"

	"github.com/wuhumanxiu/agent-native-im/internal/model"
)

var ErrMentionNotParticipant = errors.New("mentioned entity is not a participant")

type ResolverStore interface {
	GetEntityByPublicID(ctx context.Context, publicID string) (*model.Entity, error)
	GetEntitiesByIDs(ctx context.Context, ids []int64) ([]*model.Entity, error)
	ListParticipants(ctx context.Context, conversationID int64) ([]*model.Participant, error)
	IsParticipant(ctx context.Context, conversationID, entityID int64) (bool, error)
}

type ResolveResult struct {
	EntityIDs         []int64
	MentionRefs       []model.MentionRef
	AssignedEntityIDs []int64
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
	result, err := Resolve(ctx, store, conversationID, legacyIDs, publicIDs, nil, nil)
	if err != nil {
		return nil, err
	}
	return result.EntityIDs, nil
}

func Resolve(
	ctx context.Context,
	store ResolverStore,
	conversationID int64,
	legacyIDs []int64,
	publicIDs []string,
	refs []model.MentionRef,
	assignedPublicIDs *[]string,
) (*ResolveResult, error) {
	resolved := make([]int64, 0, len(legacyIDs)+len(publicIDs)+len(refs))
	resolvedRefs := make([]model.MentionRef, 0, len(legacyIDs)+len(publicIDs)+len(refs))
	seen := map[int64]struct{}{}
	entityByID := map[int64]*model.Entity{}

	addEntity := func(entity *model.Entity, ref model.MentionRef) error {
		if entity == nil {
			return ErrMentionNotParticipant
		}
		entityID := entity.ID
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
		entityByID[entityID] = entity
		resolved = append(resolved, entityID)
		resolvedRefs = append(resolvedRefs, normalizeRef(entity, ref))
		return nil
	}

	for _, ref := range refs {
		entity, err := resolveRef(ctx, store, conversationID, ref)
		if err != nil || entity == nil {
			return nil, ErrMentionNotParticipant
		}
		if err := addEntity(entity, ref); err != nil {
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
		if err := addEntity(entity, model.MentionRef{PublicID: publicID}); err != nil {
			return nil, err
		}
	}

	for _, entityID := range legacyIDs {
		entities, err := store.GetEntitiesByIDs(ctx, []int64{entityID})
		if err != nil || len(entities) == 0 || entities[0] == nil {
			return nil, ErrMentionNotParticipant
		}
		if err := addEntity(entities[0], model.MentionRef{}); err != nil {
			return nil, err
		}
	}

	assigned, err := resolveAssignedPublicIDs(ctx, store, assignedPublicIDs, seen)
	if err != nil {
		return nil, err
	}
	if assignedPublicIDs == nil {
		assigned = append([]int64(nil), resolved...)
	}

	return &ResolveResult{
		EntityIDs:         resolved,
		MentionRefs:       resolvedRefs,
		AssignedEntityIDs: assigned,
	}, nil
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

func PublicRefsForEntityIDs(ctx context.Context, store ResolverStore, entityIDs []int64) []model.MentionRef {
	entities := MentionedEntities(ctx, store, entityIDs)
	refs := make([]model.MentionRef, 0, len(entities))
	for _, entity := range entities {
		refs = append(refs, normalizeRef(entity, model.MentionRef{}))
	}
	return refs
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

func resolveAssignedPublicIDs(
	ctx context.Context,
	store ResolverStore,
	assignedPublicIDs *[]string,
	mentioned map[int64]struct{},
) ([]int64, error) {
	if assignedPublicIDs == nil {
		return nil, nil
	}
	assigned := make([]int64, 0, len(*assignedPublicIDs))
	seen := map[int64]struct{}{}
	for _, raw := range *assignedPublicIDs {
		publicID := strings.TrimSpace(raw)
		if publicID == "" {
			continue
		}
		entity, err := store.GetEntityByPublicID(ctx, publicID)
		if err != nil || entity == nil {
			return nil, ErrMentionNotParticipant
		}
		if _, ok := mentioned[entity.ID]; !ok {
			return nil, ErrMentionNotParticipant
		}
		if _, ok := seen[entity.ID]; ok {
			continue
		}
		seen[entity.ID] = struct{}{}
		assigned = append(assigned, entity.ID)
	}
	return assigned, nil
}

func resolveRef(ctx context.Context, store ResolverStore, conversationID int64, ref model.MentionRef) (*model.Entity, error) {
	publicID := strings.TrimSpace(ref.PublicID)
	if publicID != "" {
		return store.GetEntityByPublicID(ctx, publicID)
	}
	handle := strings.TrimSpace(strings.TrimPrefix(ref.Handle, "@"))
	if handle == "" {
		handle = strings.TrimSpace(strings.TrimPrefix(ref.Text, "@"))
	}
	if handle == "" {
		return nil, ErrMentionNotParticipant
	}
	return resolveHandle(ctx, store, conversationID, handle)
}

func resolveHandle(ctx context.Context, store ResolverStore, conversationID int64, handle string) (*model.Entity, error) {
	participants, err := store.ListParticipants(ctx, conversationID)
	if err != nil {
		return nil, err
	}
	needle := strings.ToLower(strings.TrimSpace(strings.TrimPrefix(handle, "@")))
	var matched *model.Entity
	for _, participant := range participants {
		if participant == nil || participant.Entity == nil {
			continue
		}
		entity := participant.Entity
		for _, alias := range aliasesFor(entity) {
			if strings.ToLower(alias) != needle {
				continue
			}
			if matched != nil && matched.ID != entity.ID {
				return nil, ErrMentionNotParticipant
			}
			matched = entity
		}
	}
	if matched == nil {
		return nil, ErrMentionNotParticipant
	}
	return matched, nil
}

func aliasesFor(entity *model.Entity) []string {
	if entity == nil {
		return nil
	}
	aliases := []string{}
	for _, value := range []string{
		entity.BotID,
		entity.Name,
		entity.DisplayName,
		entity.PublicID,
	} {
		value = strings.TrimSpace(strings.TrimPrefix(value, "@"))
		if value != "" {
			aliases = append(aliases, value)
		}
	}
	return aliases
}

func normalizeRef(entity *model.Entity, ref model.MentionRef) model.MentionRef {
	if entity == nil {
		return ref
	}
	ref.PublicID = strings.TrimSpace(ref.PublicID)
	if ref.PublicID == "" {
		ref.PublicID = entity.PublicID
	}
	if strings.TrimSpace(ref.Handle) == "" {
		ref.Handle = preferredHandle(entity)
	}
	if strings.TrimSpace(ref.DisplayName) == "" {
		ref.DisplayName = entity.DisplayName
		if ref.DisplayName == "" {
			ref.DisplayName = entity.Name
		}
	}
	if strings.TrimSpace(ref.EntityType) == "" {
		ref.EntityType = string(entity.EntityType)
	}
	if strings.TrimSpace(ref.Text) == "" {
		if ref.Handle != "" {
			ref.Text = "@" + strings.TrimPrefix(ref.Handle, "@")
		} else if ref.DisplayName != "" {
			ref.Text = "@" + ref.DisplayName
		}
	}
	return ref
}

func preferredHandle(entity *model.Entity) string {
	if entity == nil {
		return ""
	}
	if strings.TrimSpace(entity.BotID) != "" {
		return entity.BotID
	}
	if strings.TrimSpace(entity.Name) != "" {
		return entity.Name
	}
	return entity.PublicID
}
