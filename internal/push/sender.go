package push

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strings"

	webpush "github.com/SherClockHolmes/webpush-go"
	"github.com/wzfukui/agent-native-im/internal/config"
	"github.com/wzfukui/agent-native-im/internal/model"
	"github.com/wzfukui/agent-native-im/internal/store"
)

// Sender handles sending Web Push notifications.
type Sender struct {
	store  store.Store
	config *config.Config
}

// NewSender creates a new push notification sender.
func NewSender(st store.Store, cfg *config.Config) *Sender {
	return &Sender{store: st, config: cfg}
}

// Payload is the data sent in a push notification.
type Payload struct {
	Title          string `json:"title"`
	Body           string `json:"body"`
	Kind           string `json:"kind,omitempty"`
	Path           string `json:"path,omitempty"`
	ConversationID int64  `json:"conversation_id"`
	MessageID      int64  `json:"message_id"`
}

// SendToEntity sends push notifications to all subscribed devices of an entity.
func (s *Sender) SendToEntity(ctx context.Context, entityID int64, payload Payload) {
	subs, err := s.store.GetPushSubscriptionsByEntity(ctx, entityID)
	if err != nil || len(subs) == 0 {
		return
	}

	payloadBytes, _ := json.Marshal(payload)

	for _, sub := range subs {
		if err := s.sendToSubscription(ctx, entityID, sub, payloadBytes, payload); err != nil {
			slog.Error("push: failed to send", "entity_id", entityID, "provider", sub.Provider, "endpoint", shortenEndpoint(sub.Endpoint), "error", err)
			_ = s.store.UpdatePushSubscriptionDeliveryStatus(ctx, sub.ID, err.Error(), false)
		}
	}
}

func (s *Sender) sendToSubscription(ctx context.Context, entityID int64, sub *model.PushSubscription, payloadBytes []byte, payload Payload) error {
	switch strings.TrimSpace(sub.Provider) {
	case "", model.PushProviderWebPush:
		return s.sendWebPush(ctx, entityID, sub, payloadBytes)
	case model.PushProviderExpo:
		return s.sendExpoPush(ctx, entityID, sub, payload)
	default:
		return fmt.Errorf("unsupported provider %q", sub.Provider)
	}
}

func (s *Sender) sendWebPush(ctx context.Context, entityID int64, sub *model.PushSubscription, payloadBytes []byte) error {
	if s.config.VAPIDPublicKey == "" || s.config.VAPIDPrivateKey == "" {
		return fmt.Errorf("webpush not configured")
	}
	resp, err := webpush.SendNotification(payloadBytes, &webpush.Subscription{
		Endpoint: sub.Endpoint,
		Keys: webpush.Keys{
			P256dh: sub.KeyP256DH,
			Auth:   sub.KeyAuth,
		},
	}, &webpush.Options{
		VAPIDPublicKey:  s.config.VAPIDPublicKey,
		VAPIDPrivateKey: s.config.VAPIDPrivateKey,
		Subscriber:      s.config.VAPIDSubject,
	})
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusGone {
		_ = s.store.DeletePushSubscription(ctx, entityID, sub.Endpoint)
		return fmt.Errorf("expired webpush subscription")
	}
	if resp.StatusCode >= 400 {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 1024))
		return fmt.Errorf("webpush status %d: %s", resp.StatusCode, strings.TrimSpace(string(body)))
	}
	_ = s.store.UpdatePushSubscriptionDeliveryStatus(ctx, sub.ID, "", true)
	slog.Info("push: sent", "entity_id", entityID, "provider", sub.Provider, "status", resp.StatusCode, "endpoint", shortenEndpoint(sub.Endpoint))
	return nil
}

func (s *Sender) sendExpoPush(ctx context.Context, entityID int64, sub *model.PushSubscription, payload Payload) error {
	bodyBytes, _ := json.Marshal([]map[string]any{{
		"to":    sub.Endpoint,
		"title": payload.Title,
		"body":  payload.Body,
		"data": map[string]any{
			"kind":            payload.Kind,
			"path":            payload.Path,
			"conversation_id": payload.ConversationID,
			"message_id":      payload.MessageID,
		},
	}})
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, "https://exp.host/--/api/v2/push/send", bytes.NewReader(bodyBytes))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	if token := strings.TrimSpace(s.config.ExpoAccessToken); token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
	if resp.StatusCode >= 400 {
		return fmt.Errorf("expo status %d: %s", resp.StatusCode, strings.TrimSpace(string(body)))
	}
	var parsed struct {
		Data []struct {
			Status  string `json:"status"`
			Message string `json:"message"`
			Details struct {
				Error string `json:"error"`
			} `json:"details"`
		} `json:"data"`
	}
	if err := json.Unmarshal(body, &parsed); err == nil && len(parsed.Data) > 0 {
		item := parsed.Data[0]
		if item.Status == "error" {
			if item.Details.Error == "DeviceNotRegistered" {
				_ = s.store.DeletePushSubscription(ctx, entityID, sub.Endpoint)
			}
			return fmt.Errorf("expo push error: %s %s", item.Details.Error, strings.TrimSpace(item.Message))
		}
	}
	_ = s.store.UpdatePushSubscriptionDeliveryStatus(ctx, sub.ID, "", true)
	slog.Info("push: sent", "entity_id", entityID, "provider", sub.Provider, "endpoint", shortenEndpoint(sub.Endpoint))
	return nil
}

func shortenEndpoint(endpoint string) string {
	if len(endpoint) <= 40 {
		return endpoint
	}
	return endpoint[:40]
}
