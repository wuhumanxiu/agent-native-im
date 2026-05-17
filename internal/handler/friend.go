package handler

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/wuhumanxiu/agent-native-im/internal/auth"
	"github.com/wuhumanxiu/agent-native-im/internal/model"
	"github.com/wuhumanxiu/agent-native-im/internal/ws"
)

func orderedFriendEntityPair(entityA, entityB int64) (int64, int64) {
	if entityA < entityB {
		return entityA, entityB
	}
	return entityB, entityA
}

func entityDisplayName(entity *model.Entity) string {
	if entity == nil {
		return "Someone"
	}
	if strings.TrimSpace(entity.DisplayName) != "" {
		return entity.DisplayName
	}
	if strings.TrimSpace(entity.Name) != "" {
		return entity.Name
	}
	if strings.TrimSpace(entity.BotID) != "" {
		return entity.BotID
	}
	if strings.TrimSpace(entity.PublicID) != "" {
		return entity.PublicID
	}
	return "Someone"
}

func (s *Server) resolveActingEntity(c *gin.Context, requestedID int64) (*model.Entity, bool) {
	ctx := c.Request.Context()
	authEntityID := auth.GetEntityID(c)
	authEntityType := auth.GetEntityType(c)

	if requestedID == 0 || requestedID == authEntityID {
		entity, err := s.Store.GetEntityByID(ctx, authEntityID)
		if err != nil || entity == nil {
			FailWithCode(c, http.StatusNotFound, ErrCodeEntityNotFound, "entity not found")
			return nil, false
		}
		return entity, true
	}

	if authEntityType != model.EntityUser {
		FailWithCode(c, http.StatusForbidden, ErrCodePermDenied, "cannot act as another entity")
		return nil, false
	}

	entity, err := s.Store.GetEntityByID(ctx, requestedID)
	if err != nil || entity == nil {
		FailWithCode(c, http.StatusNotFound, ErrCodeEntityNotFound, "entity not found")
		return nil, false
	}
	if entity.OwnerID == nil || *entity.OwnerID != authEntityID {
		FailWithCode(c, http.StatusForbidden, ErrCodePermNotOwner, "not the owner of this entity")
		return nil, false
	}
	return entity, true
}

func (s *Server) resolveActingEntityByInputs(c *gin.Context, requestedID int64, requestedPublicID string) (*model.Entity, bool) {
	entity, ok := s.resolveEntityPublicIDInput(c, requestedID, requestedPublicID, "acting")
	if !ok {
		return nil, false
	}
	if entity == nil {
		return s.resolveActingEntity(c, 0)
	}
	return s.resolveActingEntity(c, entity.ID)
}

func (s *Server) areFriends(ctx *gin.Context, entityA, entityB int64) bool {
	friendship, err := s.Store.GetFriendship(ctx.Request.Context(), entityA, entityB)
	return err == nil && friendship != nil
}

func (s *Server) canStartDirectConversation(c *gin.Context, initiator *model.Entity, target *model.Entity) bool {
	if initiator == nil || target == nil {
		return false
	}
	if initiator.ID == target.ID {
		return false
	}
	if initiator.OwnerID != nil && *initiator.OwnerID == target.ID {
		return true
	}
	if target.OwnerID != nil && *target.OwnerID == initiator.ID {
		return true
	}
	if initiator.OwnerID != nil && target.OwnerID != nil && *initiator.OwnerID == *target.OwnerID {
		return true
	}
	if s.areFriends(c, initiator.ID, target.ID) {
		return true
	}
	return (target.EntityType == model.EntityBot || target.EntityType == model.EntityService) &&
		target.DirectMessagePolicy == model.DirectMessagePolicyPlatformEntities
}

func canReceiveFriendRequest(target *model.Entity) bool {
	if target == nil {
		return false
	}
	if target.EntityType != model.EntityBot && target.EntityType != model.EntityService {
		return true
	}
	return target.FriendRequestPolicy == model.FriendRequestPolicyPlatformEntities
}

// GET /entities/discover?q=...
func (s *Server) HandleSearchDiscoverableEntities(c *gin.Context) {
	query := strings.TrimSpace(c.Query("q"))
	if query == "" {
		Fail(c, http.StatusBadRequest, "query parameter 'q' is required")
		return
	}
	limit := 20
	if raw := strings.TrimSpace(c.Query("limit")); raw != "" {
		if parsed, err := strconv.Atoi(raw); err == nil && parsed > 0 && parsed <= 100 {
			limit = parsed
		}
	}
	entities, err := s.Store.SearchDiscoverableEntities(c.Request.Context(), query, limit, auth.GetEntityID(c))
	if err != nil {
		Fail(c, http.StatusInternalServerError, "failed to search entities")
		return
	}
	s.attachEntitiesIdentity(c.Request.Context(), entities)
	if entities == nil {
		entities = []*model.Entity{}
	}
	OK(c, http.StatusOK, entities)
}

// GET /friends?entity_id=
func (s *Server) HandleListFriends(c *gin.Context) {
	entityID := parseOptionalEntityID(c.Query("entity_id"))
	entity, ok := s.resolveActingEntityByInputs(c, entityID, c.Query("public_id"))
	if !ok {
		return
	}
	friends, err := s.Store.ListFriends(c.Request.Context(), entity.ID)
	if err != nil {
		Fail(c, http.StatusInternalServerError, "failed to list friends")
		return
	}
	s.attachEntitiesIdentity(c.Request.Context(), friends)
	if friends == nil {
		friends = []*model.Entity{}
	}
	OK(c, http.StatusOK, friends)
}

// GET /friends/requests
func (s *Server) HandleListFriendRequests(c *gin.Context) {
	entityID := parseOptionalEntityID(c.Query("entity_id"))
	entity, ok := s.resolveActingEntityByInputs(c, entityID, c.Query("public_id"))
	if !ok {
		return
	}
	direction := strings.TrimSpace(c.Query("direction"))
	status := strings.TrimSpace(c.Query("status"))
	reqs, err := s.Store.ListFriendRequestsByEntity(c.Request.Context(), entity.ID, direction, status)
	if err != nil {
		Fail(c, http.StatusInternalServerError, "failed to list friend requests")
		return
	}
	s.attachFriendRequestsIdentity(c.Request.Context(), reqs)
	if reqs == nil {
		reqs = []*model.FriendRequest{}
	}
	OK(c, http.StatusOK, reqs)
}

// POST /friends/requests
func (s *Server) HandleCreateFriendRequest(c *gin.Context) {
	var req struct {
		SourceEntityID int64  `json:"source_entity_id"`
		SourcePublicID string `json:"source_public_id"`
		TargetEntityID int64  `json:"target_entity_id"`
		TargetPublicID string `json:"target_public_id"`
		Message        string `json:"message"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		Fail(c, http.StatusBadRequest, err.Error())
		return
	}

	source, ok := s.resolveActingEntityByInputs(c, req.SourceEntityID, req.SourcePublicID)
	if !ok {
		return
	}
	target, ok := s.resolveEntityPublicIDInput(c, req.TargetEntityID, req.TargetPublicID, "target")
	if !ok {
		return
	}
	if target == nil || target.ID == source.ID {
		FailWithCode(c, http.StatusBadRequest, ErrCodeValidationField, "target_entity_id or target_public_id is invalid")
		return
	}
	if target.Status != "active" {
		FailWithCode(c, http.StatusBadRequest, ErrCodeStateBadTransition, "target entity is not active")
		return
	}
	normalizeInteractionPolicies(target)
	if !canReceiveFriendRequest(target) {
		FailWithCode(c, http.StatusForbidden, ErrCodePermDenied, "target entity does not accept friend requests")
		return
	}
	if s.areFriends(c, source.ID, target.ID) {
		friendship, _ := s.Store.GetFriendship(c.Request.Context(), source.ID, target.ID)
		OK(c, http.StatusOK, gin.H{"friendship": friendship, "already_friends": true})
		return
	}
	if existing, err := s.Store.FindPendingFriendRequest(c.Request.Context(), source.ID, target.ID); err == nil && existing != nil {
		OK(c, http.StatusOK, existing)
		return
	}
	if reverse, err := s.Store.FindPendingFriendRequest(c.Request.Context(), target.ID, source.ID); err == nil && reverse != nil {
		reverse.Status = model.FriendRequestAccepted
		resolver := source.ID
		reverse.ResolvedBy = &resolver
		if err := s.Store.UpdateFriendRequest(c.Request.Context(), reverse); err != nil {
			Fail(c, http.StatusInternalServerError, "failed to accept friend request")
			return
		}
		low, high := orderedFriendEntityPair(source.ID, target.ID)
		friendship := &model.Friendship{EntityLowID: low, EntityHighID: high, CreatedBy: source.ID}
		if err := s.Store.CreateFriendship(c.Request.Context(), friendship); err != nil {
			Fail(c, http.StatusInternalServerError, "failed to create friendship")
			return
		}
		s.attachEntityIdentity(c.Request.Context(), source)
		s.attachEntityIdentity(c.Request.Context(), target)
		s.attachFriendRequestIdentity(c.Request.Context(), reverse)
		_ = s.broadcastFriendRequestUpdate(c, reverse, "accepted")
		OK(c, http.StatusCreated, gin.H{"auto_accepted": true, "friendship": friendship, "request": reverse})
		return
	}

	friendReq := &model.FriendRequest{
		SourceEntityID: source.ID,
		TargetEntityID: target.ID,
		Message:        strings.TrimSpace(req.Message),
		Status:         model.FriendRequestPending,
	}
	if err := s.Store.CreateFriendRequest(c.Request.Context(), friendReq); err != nil {
		Fail(c, http.StatusInternalServerError, "failed to create friend request")
		return
	}
	friendReq.SourceEntity = source
	friendReq.TargetEntity = target
	s.attachFriendRequestIdentity(c.Request.Context(), friendReq)
	_ = s.broadcastFriendRequestCreated(c, friendReq)
	OK(c, http.StatusCreated, friendReq)
}

func (s *Server) emitFriendRequestEvent(entityID int64, eventType string, friendReq *model.FriendRequest) {
	if friendReq == nil {
		return
	}
	s.Hub.SendToEntity(entityID, ws.WSMessage{
		Type: eventType,
		Data: friendReq,
	})
}

func (s *Server) broadcastFriendRequestCreated(c *gin.Context, friendReq *model.FriendRequest) error {
	if friendReq == nil {
		return nil
	}
	actorID := friendReq.SourceEntityID
	if _, err := s.createNotification(
		c,
		friendReq.TargetEntityID,
		&actorID,
		"friend.request.received",
		"New friend request",
		fmt.Sprintf("%s sent a friend request", entityDisplayName(friendReq.SourceEntity)),
		map[string]any{
			"request_id":          friendReq.ID,
			"source_entity_id":    friendReq.SourceEntityID,
			"target_entity_id":    friendReq.TargetEntityID,
			"status":              friendReq.Status,
			"source_public_id":    friendReq.SourceEntity.PublicID,
			"target_public_id":    friendReq.TargetEntity.PublicID,
			"source_display_name": entityDisplayName(friendReq.SourceEntity),
			"target_display_name": entityDisplayName(friendReq.TargetEntity),
		},
	); err != nil {
		return err
	}
	s.emitFriendRequestEvent(friendReq.SourceEntityID, "friend.request.created", friendReq)
	s.emitFriendRequestEvent(friendReq.TargetEntityID, "friend.request.created", friendReq)
	return nil
}

func (s *Server) broadcastFriendRequestUpdate(c *gin.Context, friendReq *model.FriendRequest, action string) error {
	if friendReq == nil {
		return nil
	}
	if friendReq.SourceEntity == nil {
		friendReq.SourceEntity, _ = s.Store.GetEntityByID(c.Request.Context(), friendReq.SourceEntityID)
	}
	if friendReq.TargetEntity == nil {
		friendReq.TargetEntity, _ = s.Store.GetEntityByID(c.Request.Context(), friendReq.TargetEntityID)
	}
	s.attachEntityIdentity(c.Request.Context(), friendReq.SourceEntity)
	s.attachEntityIdentity(c.Request.Context(), friendReq.TargetEntity)

	var title string
	var body string
	switch action {
	case "accepted":
		title = "Friend request accepted"
		body = fmt.Sprintf("%s accepted your friend request", entityDisplayName(friendReq.TargetEntity))
	case "rejected":
		title = "Friend request declined"
		body = fmt.Sprintf("%s declined your friend request", entityDisplayName(friendReq.TargetEntity))
	case "canceled":
		title = "Friend request canceled"
		body = fmt.Sprintf("%s canceled a friend request", entityDisplayName(friendReq.SourceEntity))
	default:
		title = "Friend request updated"
		body = "A friend request changed status"
	}

	var actorID *int64
	if friendReq.ResolvedBy != nil {
		actorID = friendReq.ResolvedBy
	}
	recipientID := friendReq.SourceEntityID
	if action == "canceled" {
		recipientID = friendReq.TargetEntityID
	}
	if _, err := s.createNotification(
		c,
		recipientID,
		actorID,
		"friend.request."+action,
		title,
		body,
		map[string]any{
			"request_id":          friendReq.ID,
			"source_entity_id":    friendReq.SourceEntityID,
			"target_entity_id":    friendReq.TargetEntityID,
			"status":              friendReq.Status,
			"resolved_by":         friendReq.ResolvedBy,
			"source_public_id":    friendReq.SourceEntity.PublicID,
			"target_public_id":    friendReq.TargetEntity.PublicID,
			"source_display_name": entityDisplayName(friendReq.SourceEntity),
			"target_display_name": entityDisplayName(friendReq.TargetEntity),
		},
	); err != nil {
		return err
	}
	s.emitFriendRequestEvent(friendReq.SourceEntityID, "friend.request.updated", friendReq)
	s.emitFriendRequestEvent(friendReq.TargetEntityID, "friend.request.updated", friendReq)
	return nil
}

func (s *Server) resolveFriendRequestTarget(c *gin.Context) (*model.FriendRequest, *model.Entity, bool) {
	reqID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		Fail(c, http.StatusBadRequest, "invalid friend request id")
		return nil, nil, false
	}
	friendReq, err := s.Store.GetFriendRequestByID(c.Request.Context(), reqID)
	if err != nil || friendReq == nil {
		Fail(c, http.StatusNotFound, "friend request not found")
		return nil, nil, false
	}
	actingEntityID := parseOptionalEntityID(c.Query("entity_id"))
	entity, ok := s.resolveActingEntityByInputs(c, actingEntityID, c.Query("public_id"))
	if !ok {
		return nil, nil, false
	}
	return friendReq, entity, true
}

// POST /friends/requests/:id/accept
func (s *Server) HandleAcceptFriendRequest(c *gin.Context) {
	friendReq, actor, ok := s.resolveFriendRequestTarget(c)
	if !ok {
		return
	}
	if friendReq.TargetEntityID != actor.ID {
		FailWithCode(c, http.StatusForbidden, ErrCodePermDenied, "cannot accept this request")
		return
	}
	if friendReq.Status != model.FriendRequestPending {
		if friendReq.Status == model.FriendRequestAccepted {
			friendship, err := s.ensureFriendshipForFriendRequest(c, friendReq, actor.ID)
			if err != nil {
				Fail(c, http.StatusInternalServerError, "failed to create friendship")
				return
			}
			s.attachFriendRequestIdentity(c.Request.Context(), friendReq)
			OK(c, http.StatusOK, gin.H{"request": friendReq, "friendship": friendship, "already_accepted": true})
			return
		}
		FailWithCode(c, http.StatusBadRequest, ErrCodeStateBadTransition, "friend request is not pending")
		return
	}
	friendReq.Status = model.FriendRequestAccepted
	friendReq.ResolvedBy = &actor.ID
	if err := s.Store.UpdateFriendRequest(c.Request.Context(), friendReq); err != nil {
		Fail(c, http.StatusInternalServerError, "failed to accept friend request")
		return
	}
	friendship, err := s.ensureFriendshipForFriendRequest(c, friendReq, actor.ID)
	if err != nil {
		Fail(c, http.StatusInternalServerError, "failed to create friendship")
		return
	}
	s.attachFriendRequestIdentity(c.Request.Context(), friendReq)
	_ = s.broadcastFriendRequestUpdate(c, friendReq, "accepted")
	OK(c, http.StatusOK, gin.H{"request": friendReq, "friendship": friendship})
}

func (s *Server) ensureFriendshipForFriendRequest(c *gin.Context, friendReq *model.FriendRequest, createdBy int64) (*model.Friendship, error) {
	low, high := orderedFriendEntityPair(friendReq.SourceEntityID, friendReq.TargetEntityID)
	friendship := &model.Friendship{EntityLowID: low, EntityHighID: high, CreatedBy: createdBy}
	if err := s.Store.CreateFriendship(c.Request.Context(), friendship); err != nil {
		return nil, err
	}
	existing, err := s.Store.GetFriendship(c.Request.Context(), friendReq.SourceEntityID, friendReq.TargetEntityID)
	if err != nil || existing == nil {
		return friendship, err
	}
	return existing, nil
}

func (s *Server) updateFriendRequestStatus(c *gin.Context, expectedEntityID int64, status model.FriendRequestStatus, message string) {
	friendReq, actor, ok := s.resolveFriendRequestTarget(c)
	if !ok {
		return
	}
	if friendReq.Status != model.FriendRequestPending {
		FailWithCode(c, http.StatusBadRequest, ErrCodeStateBadTransition, "friend request is not pending")
		return
	}
	if expectedEntityID == friendReq.TargetEntityID && friendReq.TargetEntityID != actor.ID {
		FailWithCode(c, http.StatusForbidden, ErrCodePermDenied, "cannot update this request")
		return
	}
	if expectedEntityID == friendReq.SourceEntityID && friendReq.SourceEntityID != actor.ID {
		FailWithCode(c, http.StatusForbidden, ErrCodePermDenied, "cannot update this request")
		return
	}
	friendReq.Status = status
	friendReq.ResolvedBy = &actor.ID
	if err := s.Store.UpdateFriendRequest(c.Request.Context(), friendReq); err != nil {
		Fail(c, http.StatusInternalServerError, message)
		return
	}
	action := string(status)
	_ = s.broadcastFriendRequestUpdate(c, friendReq, action)
	OK(c, http.StatusOK, friendReq)
}

// POST /friends/requests/:id/reject
func (s *Server) HandleRejectFriendRequest(c *gin.Context) {
	friendReq, _, ok := s.resolveFriendRequestTarget(c)
	if !ok {
		return
	}
	s.updateFriendRequestStatus(c, friendReq.TargetEntityID, model.FriendRequestRejected, "failed to reject friend request")
}

// POST /friends/requests/:id/cancel
func (s *Server) HandleCancelFriendRequest(c *gin.Context) {
	friendReq, _, ok := s.resolveFriendRequestTarget(c)
	if !ok {
		return
	}
	s.updateFriendRequestStatus(c, friendReq.SourceEntityID, model.FriendRequestCanceled, "failed to cancel friend request")
}

// DELETE /friends/:entityId?entity_id=
func (s *Server) HandleDeleteFriend(c *gin.Context) {
	targetID, err := strconv.ParseInt(c.Param("entityId"), 10, 64)
	if err != nil {
		Fail(c, http.StatusBadRequest, "invalid target entity id")
		return
	}
	actingID := parseOptionalEntityID(c.Query("entity_id"))
	source, ok := s.resolveActingEntityByInputs(c, actingID, c.Query("public_id"))
	if !ok {
		return
	}
	targetEntity, ok := s.resolveEntityPublicIDInput(c, targetID, c.Query("target_public_id"), "target")
	if !ok {
		return
	}
	if targetEntity == nil {
		FailWithCode(c, http.StatusBadRequest, ErrCodeValidationField, "target entity is required")
		return
	}
	if targetEntity != nil {
		targetID = targetEntity.ID
	}
	if err := s.Store.DeleteFriendship(c.Request.Context(), source.ID, targetID); err != nil {
		Fail(c, http.StatusInternalServerError, "failed to delete friendship")
		return
	}
	OK(c, http.StatusOK, gin.H{"entity_id": source.ID, "public_id": source.PublicID, "removed_friend_id": targetID, "removed_friend_public_id": targetEntity.PublicID})
}

func normalizeDiscoverability(entity *model.Entity) {
	if entity == nil {
		return
	}
	if entity.Discoverability != "" {
		return
	}
	meta := map[string]any{}
	if len(entity.Metadata) > 0 {
		_ = json.Unmarshal(entity.Metadata, &meta)
	}
	if value, ok := meta["discoverability"].(string); ok && value != "" {
		entity.Discoverability = value
		return
	}
	entity.Discoverability = "private"
}

func normalizeInteractionPolicies(entity *model.Entity) {
	if entity == nil {
		return
	}
	if strings.TrimSpace(entity.FriendRequestPolicy) == "" {
		entity.FriendRequestPolicy = model.FriendRequestPolicyPlatformEntities
	}
	if strings.TrimSpace(entity.DirectMessagePolicy) == "" {
		if entity.AllowNonFriendChat {
			entity.DirectMessagePolicy = model.DirectMessagePolicyPlatformEntities
		} else {
			entity.DirectMessagePolicy = model.DirectMessagePolicyFriendsOnly
		}
	}
	entity.AllowNonFriendChat = entity.DirectMessagePolicy == model.DirectMessagePolicyPlatformEntities
}

func validateDiscoverability(value string) bool {
	switch value {
	case "", "private", "platform_public", "external_public":
		return true
	default:
		return false
	}
}

func validateFriendRequestPolicy(value string) bool {
	switch value {
	case "", model.FriendRequestPolicyNobody, model.FriendRequestPolicyPlatformEntities:
		return true
	default:
		return false
	}
}

func validateDirectMessagePolicy(value string) bool {
	switch value {
	case "", model.DirectMessagePolicyFriendsOnly, model.DirectMessagePolicyPlatformEntities:
		return true
	default:
		return false
	}
}
