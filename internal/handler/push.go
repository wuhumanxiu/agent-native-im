package handler

import (
	"fmt"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/wzfukui/agent-native-im/internal/auth"
	"github.com/wzfukui/agent-native-im/internal/model"
)

// HandleRegisterPush registers a push subscription for the authenticated entity.
func (s *Server) HandleRegisterPush(c *gin.Context) {
	var req struct {
		Provider  string `json:"provider"`
		Platform  string `json:"platform"`
		Endpoint  string `json:"endpoint" binding:"required"`
		KeyP256DH string `json:"key_p256dh"`
		KeyAuth   string `json:"key_auth"`
		DeviceID  string `json:"device_id"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		Fail(c, http.StatusBadRequest, "endpoint is required")
		return
	}
	provider := strings.TrimSpace(req.Provider)
	if provider == "" {
		provider = model.PushProviderWebPush
	}
	switch provider {
	case model.PushProviderWebPush:
		if strings.TrimSpace(req.KeyP256DH) == "" || strings.TrimSpace(req.KeyAuth) == "" {
			Fail(c, http.StatusBadRequest, "webpush subscriptions require key_p256dh and key_auth")
			return
		}
	case model.PushProviderExpo:
		// Expo tokens use endpoint as the token and do not require keys.
	default:
		Fail(c, http.StatusBadRequest, fmt.Sprintf("unsupported push provider %q", provider))
		return
	}

	entityID := auth.GetEntityID(c)
	sub := &model.PushSubscription{
		EntityID:  entityID,
		Provider:  provider,
		Platform:  strings.TrimSpace(req.Platform),
		DeviceID:  strings.TrimSpace(req.DeviceID),
		Endpoint:  strings.TrimSpace(req.Endpoint),
		KeyP256DH: strings.TrimSpace(req.KeyP256DH),
		KeyAuth:   strings.TrimSpace(req.KeyAuth),
	}

	if err := s.Store.CreatePushSubscription(c.Request.Context(), sub); err != nil {
		Fail(c, http.StatusInternalServerError, "failed to register push subscription")
		return
	}

	OK(c, http.StatusCreated, gin.H{"message": "push subscription registered"})
}

// HandleUnregisterPush removes a push subscription.
func (s *Server) HandleUnregisterPush(c *gin.Context) {
	var req struct {
		Endpoint string `json:"endpoint" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		Fail(c, http.StatusBadRequest, "endpoint is required")
		return
	}

	entityID := auth.GetEntityID(c)
	_ = s.Store.DeletePushSubscription(c.Request.Context(), entityID, req.Endpoint)
	OK(c, http.StatusOK, gin.H{"message": "push subscription removed"})
}

// HandleGetVAPIDKey returns the VAPID public key for push subscription.
func (s *Server) HandleGetVAPIDKey(c *gin.Context) {
	if s.Config.VAPIDPublicKey == "" {
		Fail(c, http.StatusNotFound, "push notifications not configured")
		return
	}
	OK(c, http.StatusOK, gin.H{"public_key": s.Config.VAPIDPublicKey})
}
