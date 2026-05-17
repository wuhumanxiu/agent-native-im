package handler

import (
	"context"
	"encoding/json"
	"log/slog"

	"github.com/google/uuid"
	"github.com/wuhumanxiu/agent-native-im/internal/model"
)

const entityPublicIDKey = "public_id"

func ensureEntityIdentity(entity *model.Entity) (changed bool, err error) {
	meta := map[string]any{}
	if len(entity.Metadata) > 0 {
		if err := json.Unmarshal(entity.Metadata, &meta); err != nil {
			return false, err
		}
	}

	storedMetaPublicID, _ := meta[entityPublicIDKey].(string)
	switch {
	case entity.PublicID != "":
		if storedMetaPublicID != entity.PublicID {
			meta[entityPublicIDKey] = entity.PublicID
			changed = true
		}
	case storedMetaPublicID != "":
		entity.PublicID = storedMetaPublicID
		changed = true
	default:
		entity.PublicID = uuid.NewString()
		meta[entityPublicIDKey] = entity.PublicID
		changed = true
	}

	if changed {
		encoded, err := json.Marshal(meta)
		if err != nil {
			return false, err
		}
		entity.Metadata = encoded
	}

	return changed, nil
}

func hydrateEntityIdentity(entity *model.Entity) {
	if entity == nil {
		return
	}
	if _, err := ensureEntityIdentity(entity); err != nil {
		slog.Warn("failed to hydrate entity identity", "entity_id", entity.ID, "error", err)
	}
}

func (s *Server) attachEntityIdentity(ctx context.Context, entity *model.Entity) {
	if entity == nil {
		return
	}
	changed, err := ensureEntityIdentity(entity)
	if err != nil {
		slog.Warn("failed to parse entity identity metadata", "entity_id", entity.ID, "error", err)
		return
	}
	if !changed {
		s.attachEntityOwnerIdentity(ctx, entity)
		return
	}
	if err := s.Store.UpdateEntity(ctx, entity); err != nil {
		slog.Warn("failed to persist entity identity", "entity_id", entity.ID, "error", err)
	}
	s.attachEntityOwnerIdentity(ctx, entity)
}

func (s *Server) attachEntityOwnerIdentity(ctx context.Context, entity *model.Entity) {
	if entity == nil || entity.OwnerID == nil || *entity.OwnerID == 0 {
		return
	}
	owner := entity.Owner
	if owner == nil {
		var err error
		owner, err = s.Store.GetEntityByID(ctx, *entity.OwnerID)
		if err != nil {
			slog.Warn("failed to load entity owner identity", "entity_id", entity.ID, "owner_id", *entity.OwnerID, "error", err)
			return
		}
	}
	if owner == nil {
		return
	}
	changed, err := ensureEntityIdentity(owner)
	if err != nil {
		slog.Warn("failed to hydrate entity owner identity", "entity_id", entity.ID, "owner_id", owner.ID, "error", err)
		return
	}
	if changed {
		if err := s.Store.UpdateEntity(ctx, owner); err != nil {
			slog.Warn("failed to persist entity owner identity", "entity_id", entity.ID, "owner_id", owner.ID, "error", err)
		}
	}
	entity.OwnerPublicID = owner.PublicID
	entity.OwnerName = owner.Name
	entity.OwnerDisplayName = owner.DisplayName
	// Keep API responses lightweight: expose only owner identity fields, not the
	// full owner entity payload with unrelated profile details.
	entity.Owner = nil
}

func (s *Server) attachEntitiesIdentity(ctx context.Context, entities []*model.Entity) {
	for _, entity := range entities {
		s.attachEntityIdentity(ctx, entity)
	}
}

func (s *Server) attachParticipantIdentity(ctx context.Context, participant *model.Participant) {
	if participant == nil {
		return
	}
	if participant.Entity == nil && participant.EntityID > 0 {
		participant.Entity, _ = s.Store.GetEntityByID(ctx, participant.EntityID)
	}
	s.attachEntityIdentity(ctx, participant.Entity)
	if participant.Entity != nil {
		participant.EntityPublicID = participant.Entity.PublicID
	}
}

func (s *Server) attachParticipantsIdentity(ctx context.Context, participants []*model.Participant) {
	for _, participant := range participants {
		s.attachParticipantIdentity(ctx, participant)
	}
}

func (s *Server) attachFriendRequestIdentity(ctx context.Context, req *model.FriendRequest) {
	if req == nil {
		return
	}
	if req.SourceEntity == nil && req.SourceEntityID > 0 {
		req.SourceEntity, _ = s.Store.GetEntityByID(ctx, req.SourceEntityID)
	}
	if req.TargetEntity == nil && req.TargetEntityID > 0 {
		req.TargetEntity, _ = s.Store.GetEntityByID(ctx, req.TargetEntityID)
	}
	s.attachEntityIdentity(ctx, req.SourceEntity)
	s.attachEntityIdentity(ctx, req.TargetEntity)
	if req.SourceEntity != nil {
		req.SourcePublicID = req.SourceEntity.PublicID
	}
	if req.TargetEntity != nil {
		req.TargetPublicID = req.TargetEntity.PublicID
	}
}

func (s *Server) attachFriendRequestsIdentity(ctx context.Context, reqs []*model.FriendRequest) {
	for _, req := range reqs {
		s.attachFriendRequestIdentity(ctx, req)
	}
}

func (s *Server) attachConversationIdentity(ctx context.Context, conv *model.Conversation) {
	if conv == nil {
		return
	}
	s.attachParticipantsIdentity(ctx, conv.Participants)
	if conv.LastMessage != nil {
		s.populateMentionPublicRefs(ctx, []*model.Message{conv.LastMessage})
	}
}

func (s *Server) attachConversationsIdentity(ctx context.Context, convs []*model.Conversation) {
	for _, conv := range convs {
		s.attachConversationIdentity(ctx, conv)
	}
}
