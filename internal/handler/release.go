package handler

import (
	"net/http"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/wuhumanxiu/agent-native-im/internal/auth"
	"github.com/wuhumanxiu/agent-native-im/internal/model"
)

func (s *Server) HandleListReleases(c *gin.Context) {
	if auth.GetEntityType(c) != model.EntityUser {
		FailWithCode(c, http.StatusForbidden, ErrCodePermDenied, "only users can list releases")
		return
	}
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "30"))
	offset, _ := strconv.Atoi(c.DefaultQuery("offset", "0"))
	channel := strings.TrimSpace(c.DefaultQuery("channel", "production"))
	filter := model.ReleaseListFilter{
		Component: strings.TrimSpace(c.Query("component")),
		Platform:  strings.TrimSpace(c.Query("platform")),
		Channel:   channel,
		Limit:     limit,
		Offset:    offset,
	}
	entityID := auth.GetEntityID(c)
	items, total, err := s.Store.ListReleases(c.Request.Context(), filter, entityID)
	if err != nil {
		Fail(c, http.StatusInternalServerError, "failed to list releases")
		return
	}
	unread, err := s.Store.CountUnreadReleases(c.Request.Context(), entityID, channel)
	if err != nil {
		Fail(c, http.StatusInternalServerError, "failed to count unread releases")
		return
	}
	OK(c, http.StatusOK, gin.H{
		"releases":     items,
		"items":        items,
		"total":        total,
		"limit":        filter.Limit,
		"offset":       filter.Offset,
		"unread_count": unread,
	})
}

func (s *Server) HandleLatestRelease(c *gin.Context) {
	if auth.GetEntityType(c) != model.EntityUser {
		FailWithCode(c, http.StatusForbidden, ErrCodePermDenied, "only users can view releases")
		return
	}
	channel := strings.TrimSpace(c.DefaultQuery("channel", "production"))
	entityID := auth.GetEntityID(c)
	release, err := s.Store.GetLatestRelease(c.Request.Context(), channel, entityID)
	if err != nil {
		FailWithCode(c, http.StatusNotFound, ErrCodeNotFound, "release not found")
		return
	}
	unread, err := s.Store.CountUnreadReleases(c.Request.Context(), entityID, channel)
	if err != nil {
		Fail(c, http.StatusInternalServerError, "failed to count unread releases")
		return
	}
	OK(c, http.StatusOK, gin.H{"release": release, "unread_count": unread})
}

func (s *Server) HandleMarkReleaseRead(c *gin.Context) {
	if auth.GetEntityType(c) != model.EntityUser {
		FailWithCode(c, http.StatusForbidden, ErrCodePermDenied, "only users can mark releases read")
		return
	}
	releaseID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || releaseID <= 0 {
		Fail(c, http.StatusBadRequest, "invalid release id")
		return
	}
	if _, err := s.Store.GetReleaseByID(c.Request.Context(), releaseID); err != nil {
		FailWithCode(c, http.StatusNotFound, ErrCodeNotFound, "release not found")
		return
	}
	if err := s.Store.MarkReleaseRead(c.Request.Context(), auth.GetEntityID(c), releaseID); err != nil {
		Fail(c, http.StatusInternalServerError, "failed to mark release read")
		return
	}
	OK(c, http.StatusOK, gin.H{"release_id": releaseID, "message": "release marked read"})
}
