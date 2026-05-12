package handler

import (
	"context"
	"net/http"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/wzfukui/agent-native-im/internal/model"
)

func (s *Server) getEntityByPublicID(ctx context.Context, publicID string) (*model.Entity, error) {
	publicID = strings.TrimSpace(publicID)
	if publicID == "" {
		return nil, nil
	}
	if _, err := uuid.Parse(publicID); err != nil {
		return nil, err
	}
	return s.Store.GetEntityByPublicID(ctx, publicID)
}

func (s *Server) resolveEntityPublicIDInput(c *gin.Context, numericID int64, publicID string, label string) (*model.Entity, bool) {
	ctx := c.Request.Context()
	publicID = strings.TrimSpace(publicID)

	var byPublic *model.Entity
	if publicID != "" {
		entity, err := s.getEntityByPublicID(ctx, publicID)
		if err != nil || entity == nil {
			FailWithCode(c, http.StatusNotFound, ErrCodeEntityNotFound, label+" public_id not found")
			return nil, false
		}
		byPublic = entity
	}

	if numericID > 0 {
		byID, err := s.Store.GetEntityByID(ctx, numericID)
		if err != nil || byID == nil {
			FailWithCode(c, http.StatusNotFound, ErrCodeEntityNotFound, label+" entity_id not found")
			return nil, false
		}
		if byPublic != nil && byPublic.ID != byID.ID {
			FailWithCode(c, http.StatusBadRequest, ErrCodeValidationField, label+" entity_id conflicts with public_id")
			return nil, false
		}
		return byID, true
	}

	if byPublic != nil {
		return byPublic, true
	}

	return nil, true
}

func (s *Server) resolvePublicIDsInput(c *gin.Context, numericIDs []int64, publicIDs []string, label string) ([]int64, bool) {
	ctx := c.Request.Context()
	resolved := make([]int64, 0, len(numericIDs)+len(publicIDs))
	seen := map[int64]struct{}{}

	add := func(entity *model.Entity) {
		if entity == nil || entity.ID <= 0 {
			return
		}
		if _, ok := seen[entity.ID]; ok {
			return
		}
		seen[entity.ID] = struct{}{}
		resolved = append(resolved, entity.ID)
	}

	publicIDToEntity := map[string]*model.Entity{}
	for _, raw := range publicIDs {
		publicID := strings.TrimSpace(raw)
		if publicID == "" {
			continue
		}
		entity, err := s.getEntityByPublicID(ctx, publicID)
		if err != nil || entity == nil {
			FailWithCode(c, http.StatusNotFound, ErrCodeEntityNotFound, label+" public_id not found")
			return nil, false
		}
		publicIDToEntity[publicID] = entity
		add(entity)
	}

	for _, id := range numericIDs {
		if id <= 0 {
			continue
		}
		entity, err := s.Store.GetEntityByID(ctx, id)
		if err != nil || entity == nil {
			FailWithCode(c, http.StatusNotFound, ErrCodeEntityNotFound, label+" entity_id not found")
			return nil, false
		}
		for publicID, byPublic := range publicIDToEntity {
			if byPublic.ID == id {
				delete(publicIDToEntity, publicID)
			}
		}
		add(entity)
	}

	return resolved, true
}

func parseOptionalEntityID(raw string) int64 {
	id, _ := strconv.ParseInt(strings.TrimSpace(raw), 10, 64)
	return id
}
