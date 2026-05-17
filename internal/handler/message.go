package handler

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/wuhumanxiu/agent-native-im/internal/auth"
	"github.com/wuhumanxiu/agent-native-im/internal/mention"
	"github.com/wuhumanxiu/agent-native-im/internal/model"
	"github.com/wuhumanxiu/agent-native-im/internal/ws"
)

// populateSenders batch-fetches sender entities for a slice of messages and
// populates each message's Sender and SenderType fields.  This replaces
// per-message GetEntityByID calls (N+1) with a single GetEntitiesByIDs query.
func (s *Server) populateSenders(ctx context.Context, msgs []*model.Message) {
	if len(msgs) == 0 {
		return
	}

	// Collect unique sender IDs
	seen := make(map[int64]struct{}, len(msgs))
	ids := make([]int64, 0, len(msgs))
	for _, msg := range msgs {
		if _, ok := seen[msg.SenderID]; !ok {
			seen[msg.SenderID] = struct{}{}
			ids = append(ids, msg.SenderID)
		}
	}

	entities, err := s.Store.GetEntitiesByIDs(ctx, ids)
	if err != nil {
		slog.Warn("populateSenders: batch fetch failed, falling back to empty", "error", err)
		return
	}

	entityMap := make(map[int64]*model.Entity, len(entities))
	for _, e := range entities {
		entityMap[e.ID] = e
	}

	for _, msg := range msgs {
		if sender, ok := entityMap[msg.SenderID]; ok {
			msg.SenderType = string(sender.EntityType)
			msg.SenderPublicID = sender.PublicID
			msg.Sender = sender
		}
	}
}

func (s *Server) populateConversationPublicIDs(ctx context.Context, msgs []*model.Message) {
	if len(msgs) == 0 {
		return
	}
	cache := map[int64]string{}
	for _, msg := range msgs {
		if msg == nil || msg.ConversationID == 0 {
			continue
		}
		if publicID, ok := cache[msg.ConversationID]; ok {
			msg.ConversationPublicID = publicID
			continue
		}
		conv, err := s.Store.GetConversation(ctx, msg.ConversationID)
		if err != nil || conv == nil {
			continue
		}
		publicID := conversationPublicID(conv)
		cache[msg.ConversationID] = publicID
		msg.ConversationPublicID = publicID
	}
}

type sendMessageRequest struct {
	ConversationID       int64               `json:"conversation_id"`
	ConversationPublicID string              `json:"conversation_public_id,omitempty"`
	ContentType          string              `json:"content_type,omitempty"`
	Layers               model.MessageLayers `json:"layers"`
	Attachments          []model.Attachment  `json:"attachments,omitempty"`
	StreamID             string              `json:"stream_id,omitempty"`
	Mentions             []int64             `json:"mentions,omitempty"`
	MentionPublicIDs     []string            `json:"mention_public_ids,omitempty"`
	MentionRefs          []model.MentionRef  `json:"mention_refs,omitempty"`
	AssignedPublicIDs    *[]string           `json:"assigned_public_ids,omitempty"`
	ReplyTo              *int64              `json:"reply_to,omitempty"`
}

func (s *Server) populateMentionPublicRefs(ctx context.Context, msgs []*model.Message) {
	for _, msg := range msgs {
		if msg == nil {
			continue
		}
		if len(msg.Mentions) > 0 {
			msg.MentionPublicIDs = mention.PublicIDsForEntityIDs(ctx, s.Store, msg.Mentions)
			msg.MentionedEntities = mention.MentionedEntities(ctx, s.Store, msg.Mentions)
			if len(msg.MentionRefs) == 0 {
				msg.MentionRefs = mention.PublicRefsForEntityIDs(ctx, s.Store, msg.Mentions)
			}
		}
		if len(msg.AssignedEntityIDs) > 0 {
			msg.AssignedPublicIDs = mention.PublicIDsForEntityIDs(ctx, s.Store, msg.AssignedEntityIDs)
		} else if msg.AssignedPublicIDs == nil {
			msg.AssignedPublicIDs = []string{}
		}
	}
}

func (s *Server) resolveMessageConversationID(ctx context.Context, numericID int64, publicID string) (int64, error) {
	publicID = strings.TrimSpace(publicID)
	var publicConv *model.Conversation
	if publicID != "" {
		conv, err := s.Store.GetConversationByPublicID(ctx, publicID)
		if err != nil || conv == nil {
			return 0, errors.New("conversation_public_id not found")
		}
		publicConv = conv
	}
	if numericID > 0 {
		if publicConv != nil && publicConv.ID != numericID {
			return 0, errors.New("conversation_id conflicts with conversation_public_id")
		}
		return numericID, nil
	}
	if publicConv != nil {
		return publicConv.ID, nil
	}
	return 0, errors.New("conversation_id or conversation_public_id is required")
}

func attachmentStoredName(raw string) string {
	value := strings.TrimSpace(raw)
	if value == "" {
		return ""
	}
	parsed, err := url.Parse(value)
	if err == nil && parsed.Path != "" {
		value = parsed.Path
	}
	if strings.HasPrefix(value, "/files/") {
		return strings.TrimPrefix(value, "/files/")
	}
	return ""
}

func (s *Server) HandleGetMessage(c *gin.Context) {
	msgID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		Fail(c, http.StatusBadRequest, "invalid message id")
		return
	}

	entityID := auth.GetEntityID(c)
	ctx := c.Request.Context()

	msg, err := s.Store.GetMessageByID(ctx, msgID)
	if err != nil {
		FailWithCode(c, http.StatusNotFound, ErrCodeMessageNotFound, "message not found")
		return
	}

	ok, err := s.Store.IsParticipant(ctx, msg.ConversationID, entityID)
	if err != nil || !ok {
		FailWithCode(c, http.StatusForbidden, ErrCodePermNotParticipant, "not a participant of this conversation")
		return
	}

	s.populateSenders(ctx, []*model.Message{msg})
	s.populateConversationPublicIDs(ctx, []*model.Message{msg})
	s.populateMentionPublicRefs(ctx, []*model.Message{msg})
	OK(c, http.StatusOK, msg)
}

func (s *Server) HandleSendMessage(c *gin.Context) {
	var req sendMessageRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		Fail(c, http.StatusBadRequest, err.Error())
		return
	}

	entityID := auth.GetEntityID(c)
	ctx := c.Request.Context()
	conversationID, err := s.resolveMessageConversationID(ctx, req.ConversationID, req.ConversationPublicID)
	if err != nil {
		FailWithCode(c, http.StatusBadRequest, ErrCodeValidationField, err.Error())
		return
	}

	// Verify participant and check observer role
	ok, err := s.Store.IsParticipant(ctx, conversationID, entityID)
	if err != nil || !ok {
		FailWithCode(c, http.StatusForbidden, ErrCodePermNotParticipant, "not a participant of this conversation")
		return
	}
	participant, err := s.Store.GetParticipant(ctx, conversationID, entityID)
	if err == nil && participant != nil && participant.Role == model.RoleObserver {
		FailWithCode(c, http.StatusForbidden, ErrCodePermObserver, "observers cannot send messages")
		return
	}

	// Resolve public UUID mentions into internal IDs and validate all mentions
	// against the current participant set.
	resolvedMentions, err := mention.Resolve(ctx, s.Store, conversationID, req.Mentions, req.MentionPublicIDs, req.MentionRefs, req.AssignedPublicIDs)
	if err != nil {
		Fail(c, http.StatusBadRequest, "mentioned entity is not a participant")
		return
	}

	contentType := model.ContentType(req.ContentType)
	if contentType == "" {
		contentType = model.ContentText
	}

	for _, att := range req.Attachments {
		storedName := attachmentStoredName(att.URL)
		if storedName == "" {
			continue
		}
		record, err := s.Store.GetFileRecordByStoredName(ctx, storedName)
		if err != nil {
			Fail(c, http.StatusBadRequest, "attachment file record not found")
			return
		}
		if record.ConversationID != nil {
			if *record.ConversationID != conversationID {
				Fail(c, http.StatusBadRequest, "attachment file belongs to a different conversation")
				return
			}
			continue
		}
		if record.UploaderID != entityID {
			Fail(c, http.StatusBadRequest, "attachment file is not owned by sender")
			return
		}
		if err := s.Store.BindFileRecordToConversation(ctx, storedName, entityID, conversationID); err != nil {
			Fail(c, http.StatusInternalServerError, "failed to bind attachment to conversation")
			return
		}
	}

	msg := &model.Message{
		ConversationID:    conversationID,
		SenderID:          entityID,
		StreamID:          req.StreamID,
		ContentType:       contentType,
		Layers:            req.Layers,
		Attachments:       req.Attachments,
		Mentions:          resolvedMentions.EntityIDs,
		MentionRefs:       resolvedMentions.MentionRefs,
		AssignedEntityIDs: resolvedMentions.AssignedEntityIDs,
		ReplyTo:           req.ReplyTo,
	}

	if err := s.Store.CreateMessage(ctx, msg); err != nil {
		Fail(c, http.StatusInternalServerError, "failed to save message")
		return
	}

	_ = s.Store.TouchConversation(ctx, conversationID)

	// Populate sender info
	sender, err := s.Store.GetEntityByID(ctx, entityID)
	if err == nil && sender != nil {
		msg.SenderType = string(sender.EntityType)
		msg.SenderPublicID = sender.PublicID
		msg.Sender = sender
	}
	s.populateConversationPublicIDs(ctx, []*model.Message{msg})
	s.populateMentionPublicRefs(ctx, []*model.Message{msg})

	// Broadcast via WebSocket
	if s.Hub != nil {
		s.Hub.BroadcastMessage(msg)
	}

	// Deliver webhooks
	if s.Webhook != nil {
		s.Webhook.DeliverAsync(msg)
	}

	// Process task_handover side effects
	if contentType == model.ContentTaskHandover {
		s.processTaskHandover(ctx, msg)
	}

	OK(c, http.StatusCreated, msg)
}

const revokeWindow = 2 * time.Minute

// HandleRevokeMessage revokes (soft-deletes) a message within the allowed time window.
func (s *Server) HandleRevokeMessage(c *gin.Context) {
	msgID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		Fail(c, http.StatusBadRequest, "invalid message id")
		return
	}

	entityID := auth.GetEntityID(c)
	ctx := c.Request.Context()

	msg, err := s.Store.GetMessageByID(ctx, msgID)
	if err != nil {
		FailWithCode(c, http.StatusNotFound, ErrCodeMessageNotFound, "message not found")
		return
	}

	if msg.SenderID != entityID {
		FailWithCode(c, http.StatusForbidden, ErrCodePermDenied, "can only revoke your own messages")
		return
	}

	if msg.RevokedAt != nil {
		FailWithCode(c, http.StatusBadRequest, ErrCodeAlreadyRevoked, "message already revoked")
		return
	}

	if time.Since(msg.CreatedAt) > revokeWindow {
		FailWithCode(c, http.StatusForbidden, ErrCodeStateExpired, "revoke window expired (2 minutes)")
		return
	}

	if err := s.Store.RevokeMessage(ctx, msgID); err != nil {
		Fail(c, http.StatusInternalServerError, "failed to revoke message")
		return
	}

	// Broadcast revocation event
	if s.Hub != nil {
		s.Hub.BroadcastEvent(msg.ConversationID, "message.revoked", gin.H{
			"message_id":      msgID,
			"conversation_id": msg.ConversationID,
		})
	}

	OK(c, http.StatusOK, "message revoked")
}

// HandleSearchMessages searches messages in a conversation using full-text search.
func (s *Server) HandleSearchMessages(c *gin.Context) {
	convID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		Fail(c, http.StatusBadRequest, "invalid conversation id")
		return
	}

	entityID := auth.GetEntityID(c)
	ctx := c.Request.Context()

	ok, err := s.Store.IsParticipant(ctx, convID, entityID)
	if err != nil || !ok {
		FailWithCode(c, http.StatusForbidden, ErrCodePermNotParticipant, "not a participant of this conversation")
		return
	}

	query := c.Query("q")
	if query == "" {
		Fail(c, http.StatusBadRequest, "query parameter 'q' is required")
		return
	}

	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "20"))
	if limit <= 0 {
		limit = 20
	} else if limit > 100 {
		limit = 100
	}

	msgs, err := s.Store.SearchMessages(ctx, convID, query, limit)
	if err != nil {
		Fail(c, http.StatusInternalServerError, "search failed")
		return
	}
	if msgs == nil {
		msgs = []*model.Message{}
	}
	s.populateSenders(ctx, msgs)
	s.populateConversationPublicIDs(ctx, msgs)
	s.populateMentionPublicRefs(ctx, msgs)

	OK(c, http.StatusOK, gin.H{
		"messages": msgs,
		"query":    query,
	})
}

// HandleInteractionResponse records a user's response to an interaction message.
func (s *Server) HandleInteractionResponse(c *gin.Context) {
	msgID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		Fail(c, http.StatusBadRequest, "invalid message id")
		return
	}

	var req struct {
		Value string `json:"value" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		Fail(c, http.StatusBadRequest, "value is required")
		return
	}

	entityID := auth.GetEntityID(c)
	ctx := c.Request.Context()

	msg, err := s.Store.GetMessageByID(ctx, msgID)
	if err != nil {
		FailWithCode(c, http.StatusNotFound, ErrCodeMessageNotFound, "message not found")
		return
	}

	// Verify the message has an interaction layer
	if msg.Layers.Interaction == nil {
		Fail(c, http.StatusBadRequest, "message has no interaction")
		return
	}

	// Verify responder is a participant
	ok, err := s.Store.IsParticipant(ctx, msg.ConversationID, entityID)
	if err != nil || !ok {
		FailWithCode(c, http.StatusForbidden, ErrCodePermNotParticipant, "not a participant of this conversation")
		return
	}

	// Broadcast interaction response event
	if s.Hub != nil {
		responder, _ := s.Store.GetEntityByID(ctx, entityID)
		responderName := "someone"
		if responder != nil {
			responderName = responder.DisplayName
			if responderName == "" {
				responderName = responder.Name
			}
		}
		s.Hub.BroadcastEvent(msg.ConversationID, "message.interaction_response", map[string]interface{}{
			"message_id":      msgID,
			"conversation_id": msg.ConversationID,
			"entity_id":       entityID,
			"entity_name":     responderName,
			"value":           req.Value,
			"responded_at":    time.Now(),
		})
	}

	OK(c, http.StatusOK, gin.H{"message_id": msgID, "value": req.Value})
}

// HandleEditMessage allows the sender to edit their own message within 5 minutes.
func (s *Server) HandleEditMessage(c *gin.Context) {
	msgID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		Fail(c, http.StatusBadRequest, "invalid message id")
		return
	}

	var req struct {
		Layers model.MessageLayers `json:"layers"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		Fail(c, http.StatusBadRequest, err.Error())
		return
	}

	entityID := auth.GetEntityID(c)
	ctx := c.Request.Context()

	msg, err := s.Store.GetMessageByID(ctx, msgID)
	if err != nil {
		FailWithCode(c, http.StatusNotFound, ErrCodeMessageNotFound, "message not found")
		return
	}

	if msg.SenderID != entityID {
		FailWithCode(c, http.StatusForbidden, ErrCodePermDenied, "can only edit your own messages")
		return
	}

	if msg.RevokedAt != nil {
		FailWithCode(c, http.StatusBadRequest, ErrCodeStateBadTransition, "cannot edit revoked message")
		return
	}

	if time.Since(msg.CreatedAt) > 5*time.Minute {
		FailWithCode(c, http.StatusForbidden, ErrCodeStateExpired, "edit window expired (5 minutes)")
		return
	}

	// Update the message layers
	msg.Layers = req.Layers

	if err := s.Store.UpdateMessage(ctx, msg); err != nil {
		Fail(c, http.StatusInternalServerError, "failed to edit message")
		return
	}

	// Broadcast edit event
	if s.Hub != nil {
		sender, _ := s.Store.GetEntityByID(ctx, entityID)
		if sender != nil {
			msg.SenderType = string(sender.EntityType)
			msg.Sender = sender
		}
		s.Hub.BroadcastEvent(msg.ConversationID, "message.updated", map[string]interface{}{
			"message": msg,
		})
	}

	OK(c, http.StatusOK, msg)
}

// HandleSendProgress broadcasts a transient progress event via WebSocket.
// The event is NOT persisted to the database — it is fire-and-forget.
func (s *Server) HandleSendProgress(c *gin.Context) {
	convID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		Fail(c, http.StatusBadRequest, "invalid conversation id")
		return
	}

	var req struct {
		StreamID string                 `json:"stream_id" binding:"required"`
		Status   map[string]interface{} `json:"status" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		Fail(c, http.StatusBadRequest, err.Error())
		return
	}

	entityID := auth.GetEntityID(c)
	ctx := c.Request.Context()

	// Verify sender is a participant
	ok, err := s.Store.IsParticipant(ctx, convID, entityID)
	if err != nil || !ok {
		FailWithCode(c, http.StatusForbidden, ErrCodePermNotParticipant, "not a participant of this conversation")
		return
	}

	// Broadcast progress event (no DB write)
	if s.Hub != nil {
		s.Hub.BroadcastProgress(convID, entityID, req.StreamID, req.Status)
	}

	OK(c, http.StatusOK, gin.H{"sent": true})
}

// HandleSendTyping broadcasts a typing indicator via WebSocket.
// The event is NOT persisted to the database — it is fire-and-forget.
func (s *Server) HandleSendTyping(c *gin.Context) {
	convID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		Fail(c, http.StatusBadRequest, "invalid conversation id")
		return
	}

	var req struct {
		IsProcessing bool   `json:"is_processing,omitempty"`
		Phase        string `json:"phase,omitempty"`
	}
	// Body is optional — a bare POST is valid (simple typing indicator)
	_ = c.ShouldBindJSON(&req)

	entityID := auth.GetEntityID(c)
	ctx := c.Request.Context()

	// Verify sender is a participant
	ok, err := s.Store.IsParticipant(ctx, convID, entityID)
	if err != nil || !ok {
		FailWithCode(c, http.StatusForbidden, ErrCodePermNotParticipant, "not a participant of this conversation")
		return
	}

	// Broadcast typing event (no DB write)
	if s.Hub != nil {
		s.Hub.BroadcastTyping(convID, entityID, req.IsProcessing, req.Phase)
	}

	OK(c, http.StatusOK, gin.H{"sent": true})
}

func (s *Server) HandleListMessages(c *gin.Context) {
	convID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		Fail(c, http.StatusBadRequest, "invalid conversation id")
		return
	}

	entityID := auth.GetEntityID(c)
	ctx := c.Request.Context()

	// Verify participant
	ok, err := s.Store.IsParticipant(ctx, convID, entityID)
	if err != nil || !ok {
		FailWithCode(c, http.StatusForbidden, ErrCodePermNotParticipant, "not a participant of this conversation")
		return
	}

	before, _ := strconv.ParseInt(c.DefaultQuery("before", "0"), 10, 64)
	sinceID, _ := strconv.ParseInt(c.DefaultQuery("since_id", "0"), 10, 64)
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "20"))
	if limit <= 0 {
		limit = 20
	} else if limit > 100 {
		limit = 100
	}

	var msgs []*model.Message
	if sinceID > 0 {
		// Catch-up mode: return messages with id > since_id (newest first)
		msgs, err = s.Store.ListMessagesSince(ctx, convID, sinceID, limit)
	} else {
		msgs, err = s.Store.ListMessages(ctx, convID, before, limit)
	}
	if err != nil {
		Fail(c, http.StatusInternalServerError, "failed to list messages")
		return
	}

	// Populate sender info for each message (batch)
	s.populateSenders(ctx, msgs)
	s.populateConversationPublicIDs(ctx, msgs)
	s.populateMentionPublicRefs(ctx, msgs)

	// Populate reactions
	if len(msgs) > 0 {
		msgIDs := make([]int64, len(msgs))
		for i, m := range msgs {
			msgIDs[i] = m.ID
		}
		reactionsMap, err := s.Store.GetReactionsByMessages(ctx, msgIDs)
		if err == nil && reactionsMap != nil {
			for _, m := range msgs {
				if r, ok := reactionsMap[m.ID]; ok {
					m.Reactions = r
				}
			}
		}
	}

	if msgs == nil {
		msgs = []*model.Message{}
	}
	hasMore := len(msgs) == limit

	OK(c, http.StatusOK, gin.H{
		"messages": msgs,
		"has_more": hasMore,
	})
}

// HandleGlobalSearchMessages searches messages across all conversations the user participates in.
func (s *Server) HandleGlobalSearchMessages(c *gin.Context) {
	entityID := auth.GetEntityID(c)
	ctx := c.Request.Context()

	query := c.Query("q")
	if len(query) < 2 {
		Fail(c, http.StatusBadRequest, "query must be at least 2 characters")
		return
	}

	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "20"))
	if limit <= 0 {
		limit = 20
	} else if limit > 50 {
		limit = 50
	}

	offset, _ := strconv.Atoi(c.DefaultQuery("offset", "0"))
	if offset < 0 {
		offset = 0
	}

	msgs, err := s.Store.GlobalSearchMessages(ctx, entityID, query, limit, offset)
	if err != nil {
		Fail(c, http.StatusInternalServerError, "search failed")
		return
	}
	if msgs == nil {
		msgs = []*model.Message{}
	}

	// Populate sender info (batch)
	s.populateSenders(ctx, msgs)
	s.populateConversationPublicIDs(ctx, msgs)
	s.populateMentionPublicRefs(ctx, msgs)

	// Enrich results with conversation title
	type searchResult struct {
		*model.Message
		ConversationTitle string `json:"conversation_title"`
	}
	results := make([]searchResult, 0, len(msgs))
	convCache := make(map[int64]string)
	for _, msg := range msgs {
		title, ok := convCache[msg.ConversationID]
		if !ok {
			conv, err := s.Store.GetConversation(ctx, msg.ConversationID)
			if err == nil && conv != nil {
				title = conv.Title
			}
			convCache[msg.ConversationID] = title
		}
		results = append(results, searchResult{
			Message:           msg,
			ConversationTitle: title,
		})
	}

	OK(c, http.StatusOK, gin.H{
		"messages": results,
		"query":    query,
	})
}

// processTaskHandover handles side-effects when a task_handover message is sent.
// It updates the referenced task's status and sends a dedicated WS event.
func (s *Server) processTaskHandover(ctx context.Context, msg *model.Message) {
	var data struct {
		HandoverType string  `json:"handover_type"`
		TaskID       *int64  `json:"task_id"`
		AssignTo     []int64 `json:"assign_to"`
	}
	if len(msg.Layers.Data) > 0 {
		if err := json.Unmarshal(msg.Layers.Data, &data); err != nil {
			slog.Error("handler: failed to parse task_handover data", "message_id", msg.ID, "error", err)
			return
		}
	}

	// Update referenced task status to handed_over
	if data.TaskID != nil {
		task, err := s.Store.GetTask(ctx, *data.TaskID)
		if err != nil {
			slog.Error("handler: task_handover lookup failed", "task_id", *data.TaskID, "error", err)
		} else if task != nil && task.ConversationID == msg.ConversationID {
			task.Status = model.TaskHandedOver
			if err := s.Store.UpdateTask(ctx, task); err != nil {
				slog.Error("handler: failed to update task to handed_over", "task_id", *data.TaskID, "error", err)
			}
		}
	}

	// Send dedicated task.handover WS event to assigned entities (cap at 50)
	if s.Hub != nil && len(data.AssignTo) > 0 {
		assignees := data.AssignTo
		if len(assignees) > 50 {
			assignees = assignees[:50]
		}
		event := ws.WSMessage{
			Type: "task.handover",
			Data: gin.H{
				"message_id":      msg.ID,
				"conversation_id": msg.ConversationID,
				"sender_id":       msg.SenderID,
				"handover_type":   data.HandoverType,
				"task_id":         data.TaskID,
			},
		}
		for _, entityID := range assignees {
			s.Hub.SendToEntity(entityID, event)
		}
	}

	if len(data.AssignTo) == 0 {
		return
	}

	conv, _ := s.Store.GetConversation(ctx, msg.ConversationID)
	sender, _ := s.Store.GetEntityByID(ctx, msg.SenderID)
	convTitle := "Conversation"
	if conv != nil && strings.TrimSpace(conv.Title) != "" {
		convTitle = conv.Title
	}
	senderPublicID := ""
	if sender != nil {
		senderPublicID = sender.PublicID
	}
	assignees := data.AssignTo
	if len(assignees) > 50 {
		assignees = assignees[:50]
	}
	for _, entityID := range assignees {
		if entityID == msg.SenderID {
			continue
		}
		_, _ = s.createNotificationForRecipient(
			ctx,
			entityID,
			&msg.SenderID,
			"task.handover",
			"Task handover received",
			getEntityDisplayName(sender)+" handed over work in "+convTitle,
			map[string]any{
				"conversation_id":        msg.ConversationID,
				"conversation_title":     convTitle,
				"conversation_public_id": conversationPublicID(conv),
				"message_id":             msg.ID,
				"sender_id":              msg.SenderID,
				"sender_public_id":       senderPublicID,
				"sender_display_name":    getEntityDisplayName(sender),
				"handover_type":          data.HandoverType,
				"task_id":                data.TaskID,
			},
		)
	}
}
