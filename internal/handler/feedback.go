package handler

import (
	"net/http"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/wzfukui/agent-native-im/internal/auth"
	"github.com/wzfukui/agent-native-im/internal/model"
)

var validFeedbackTypes = map[string]bool{
	"bug":      true,
	"feature":  true,
	"question": true,
	"account":  true,
	"other":    true,
}

var validFeedbackLevels = map[string]bool{
	"low":    true,
	"normal": true,
	"high":   true,
	"urgent": true,
}

var validFeedbackStatuses = map[string]bool{
	"open":        true,
	"triaged":     true,
	"planned":     true,
	"in_progress": true,
	"resolved":    true,
	"closed":      true,
}

func normalizeFeedbackType(value string) string {
	value = strings.TrimSpace(value)
	if validFeedbackTypes[value] {
		return value
	}
	return "other"
}

func normalizeFeedbackLevel(value string) string {
	value = strings.TrimSpace(value)
	if validFeedbackLevels[value] {
		return value
	}
	return "normal"
}

func (s *Server) isAdminEntity(c *gin.Context) bool {
	if auth.GetEntityType(c) != model.EntityUser {
		return false
	}
	entity, err := s.Store.GetEntityByID(c.Request.Context(), auth.GetEntityID(c))
	return err == nil && entity.Name == s.Config.AdminUser
}

func (s *Server) HandleCreateFeedback(c *gin.Context) {
	if auth.GetEntityType(c) != model.EntityUser {
		FailWithCode(c, http.StatusForbidden, ErrCodePermDenied, "only users can submit feedback")
		return
	}
	var req struct {
		Type        string             `json:"type"`
		Severity    string             `json:"severity"`
		Title       string             `json:"title" binding:"required"`
		Description string             `json:"description" binding:"required"`
		Contact     string             `json:"contact"`
		Attachments []model.Attachment `json:"attachments"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		Fail(c, http.StatusBadRequest, "title and description are required")
		return
	}
	req.Title = strings.TrimSpace(req.Title)
	req.Description = strings.TrimSpace(req.Description)
	if req.Title == "" || req.Description == "" {
		Fail(c, http.StatusBadRequest, "title and description are required")
		return
	}
	if len(req.Title) > 200 {
		Fail(c, http.StatusBadRequest, "title is too long")
		return
	}

	item := &model.FeedbackItem{
		PublicID:          uuid.NewString(),
		SubmitterEntityID: auth.GetEntityID(c),
		Type:              normalizeFeedbackType(req.Type),
		Severity:          normalizeFeedbackLevel(req.Severity),
		Priority:          "normal",
		Status:            "open",
		Title:             req.Title,
		Description:       req.Description,
		Contact:           strings.TrimSpace(req.Contact),
		Attachments:       req.Attachments,
	}
	if err := s.Store.CreateFeedbackItem(c.Request.Context(), item); err != nil {
		Fail(c, http.StatusInternalServerError, "failed to create feedback")
		return
	}
	created, err := s.Store.GetFeedbackItemByID(c.Request.Context(), item.ID)
	if err != nil {
		OK(c, http.StatusCreated, item)
		return
	}
	OK(c, http.StatusCreated, created)
}

func (s *Server) HandleListFeedback(c *gin.Context) {
	if auth.GetEntityType(c) != model.EntityUser {
		FailWithCode(c, http.StatusForbidden, ErrCodePermDenied, "only users can list feedback")
		return
	}
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "50"))
	offset, _ := strconv.Atoi(c.DefaultQuery("offset", "0"))
	var submitterID *int64
	if !s.isAdminEntity(c) {
		id := auth.GetEntityID(c)
		submitterID = &id
	}
	filter := model.FeedbackListFilter{
		SubmitterEntityID: submitterID,
		Status:            strings.TrimSpace(c.Query("status")),
		Type:              strings.TrimSpace(c.Query("type")),
		Query:             strings.TrimSpace(c.Query("q")),
		Limit:             limit,
		Offset:            offset,
	}
	items, total, err := s.Store.ListFeedbackItems(c.Request.Context(), filter)
	if err != nil {
		Fail(c, http.StatusInternalServerError, "failed to list feedback")
		return
	}
	OK(c, http.StatusOK, gin.H{
		"items":  items,
		"total":  total,
		"limit":  filter.Limit,
		"offset": filter.Offset,
		"admin":  submitterID == nil,
	})
}

func (s *Server) HandleGetFeedback(c *gin.Context) {
	item, includeInternal, ok := s.loadFeedbackForCurrentUser(c)
	if !ok {
		return
	}
	comments, err := s.Store.ListFeedbackComments(c.Request.Context(), item.ID, includeInternal)
	if err != nil {
		Fail(c, http.StatusInternalServerError, "failed to list feedback comments")
		return
	}
	if comments == nil {
		comments = []*model.FeedbackComment{}
	}
	links, err := s.Store.ListFeedbackReleaseLinks(c.Request.Context(), item.ID)
	if err != nil {
		Fail(c, http.StatusInternalServerError, "failed to list feedback releases")
		return
	}
	OK(c, http.StatusOK, gin.H{"item": item, "comments": comments, "releases": links, "admin": includeInternal})
}

func (s *Server) HandleCreateFeedbackComment(c *gin.Context) {
	item, isAdmin, ok := s.loadFeedbackForCurrentUser(c)
	if !ok {
		return
	}
	var req struct {
		Body        string             `json:"body" binding:"required"`
		Visibility  string             `json:"visibility"`
		Attachments []model.Attachment `json:"attachments"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		Fail(c, http.StatusBadRequest, "comment body is required")
		return
	}
	body := strings.TrimSpace(req.Body)
	if body == "" {
		Fail(c, http.StatusBadRequest, "comment body is required")
		return
	}
	visibility := "public"
	if isAdmin && strings.TrimSpace(req.Visibility) == "internal" {
		visibility = "internal"
	}
	comment := &model.FeedbackComment{
		FeedbackID:     item.ID,
		AuthorEntityID: auth.GetEntityID(c),
		Body:           body,
		Visibility:     visibility,
		Attachments:    req.Attachments,
	}
	if err := s.Store.CreateFeedbackComment(c.Request.Context(), comment); err != nil {
		Fail(c, http.StatusInternalServerError, "failed to create feedback comment")
		return
	}
	createdComments, _ := s.Store.ListFeedbackComments(c.Request.Context(), item.ID, isAdmin)
	if createdComments == nil {
		createdComments = []*model.FeedbackComment{}
	}
	OK(c, http.StatusCreated, gin.H{"comment": comment, "comments": createdComments})
}

func (s *Server) HandleAdminUpdateFeedback(c *gin.Context) {
	itemID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || itemID <= 0 {
		Fail(c, http.StatusBadRequest, "invalid feedback id")
		return
	}
	item, err := s.Store.GetFeedbackItemByID(c.Request.Context(), itemID)
	if err != nil {
		FailWithCode(c, http.StatusNotFound, ErrCodeNotFound, "feedback not found")
		return
	}
	var req struct {
		Status            *string `json:"status"`
		Priority          *string `json:"priority"`
		Severity          *string `json:"severity"`
		Type              *string `json:"type"`
		FixedInReleaseIDs []int64 `json:"fixed_in_release_ids"`
		RelatedReleaseIDs []int64 `json:"related_release_ids"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		Fail(c, http.StatusBadRequest, err.Error())
		return
	}
	if req.Status != nil {
		status := strings.TrimSpace(*req.Status)
		if !validFeedbackStatuses[status] {
			Fail(c, http.StatusBadRequest, "invalid feedback status")
			return
		}
		item.Status = status
	}
	if req.Priority != nil {
		priority := strings.TrimSpace(*req.Priority)
		if !validFeedbackLevels[priority] {
			Fail(c, http.StatusBadRequest, "invalid feedback priority")
			return
		}
		item.Priority = priority
	}
	if req.Severity != nil {
		severity := strings.TrimSpace(*req.Severity)
		if !validFeedbackLevels[severity] {
			Fail(c, http.StatusBadRequest, "invalid feedback severity")
			return
		}
		item.Severity = severity
	}
	if req.Type != nil {
		feedbackType := strings.TrimSpace(*req.Type)
		if !validFeedbackTypes[feedbackType] {
			Fail(c, http.StatusBadRequest, "invalid feedback type")
			return
		}
		item.Type = feedbackType
	}
	if err := s.Store.UpdateFeedbackItem(c.Request.Context(), item); err != nil {
		Fail(c, http.StatusInternalServerError, "failed to update feedback")
		return
	}
	if req.FixedInReleaseIDs != nil {
		if err := s.Store.ReplaceFeedbackReleaseLinks(c.Request.Context(), item.ID, "fixed", req.FixedInReleaseIDs); err != nil {
			Fail(c, http.StatusInternalServerError, "failed to link fixed release")
			return
		}
	}
	if req.RelatedReleaseIDs != nil {
		if err := s.Store.ReplaceFeedbackReleaseLinks(c.Request.Context(), item.ID, "related", req.RelatedReleaseIDs); err != nil {
			Fail(c, http.StatusInternalServerError, "failed to link related release")
			return
		}
	}
	updated, _ := s.Store.GetFeedbackItemByID(c.Request.Context(), item.ID)
	if updated != nil {
		item = updated
	}
	OK(c, http.StatusOK, item)
}

func (s *Server) loadFeedbackForCurrentUser(c *gin.Context) (*model.FeedbackItem, bool, bool) {
	if auth.GetEntityType(c) != model.EntityUser {
		FailWithCode(c, http.StatusForbidden, ErrCodePermDenied, "only users can access feedback")
		return nil, false, false
	}
	itemID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || itemID <= 0 {
		Fail(c, http.StatusBadRequest, "invalid feedback id")
		return nil, false, false
	}
	item, err := s.Store.GetFeedbackItemByID(c.Request.Context(), itemID)
	if err != nil {
		FailWithCode(c, http.StatusNotFound, ErrCodeNotFound, "feedback not found")
		return nil, false, false
	}
	isAdmin := s.isAdminEntity(c)
	if !isAdmin && item.SubmitterEntityID != auth.GetEntityID(c) {
		FailWithCode(c, http.StatusForbidden, ErrCodePermDenied, "feedback does not belong to current user")
		return nil, false, false
	}
	return item, isAdmin, true
}
