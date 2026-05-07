package handler

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/wzfukui/agent-native-im/internal/auth"
	"github.com/wzfukui/agent-native-im/internal/model"
)

const onePassTokenPath = "/token"

type onePassLoginRequest struct {
	Ticket string `json:"ticket" binding:"required"`
	State2 string `json:"state2"`
}

type onePassUserInfo struct {
	SiteID     string  `json:"site_id"`
	OpenID     string  `json:"openid"`
	UnionID    *string `json:"unionid"`
	Nickname   *string `json:"nickname"`
	HeadimgURL *string `json:"headimgurl"`
	IssuedAt   string  `json:"issued_at"`
}

func (s *Server) onePassEnabled() bool {
	cfg := s.Config
	return strings.TrimSpace(cfg.OnePassSiteID) != "" &&
		strings.TrimSpace(cfg.OnePassAK) != "" &&
		strings.TrimSpace(cfg.OnePassSK) != ""
}

// HandleOnePassConfig returns non-secret browser configuration for 1pass login.
func (s *Server) HandleOnePassConfig(c *gin.Context) {
	if !s.onePassEnabled() {
		OK(c, http.StatusOK, gin.H{"enabled": false})
		return
	}
	baseURL := strings.TrimRight(s.Config.OnePassBaseURL, "/")
	OK(c, http.StatusOK, gin.H{
		"enabled":   true,
		"site_id":   s.Config.OnePassSiteID,
		"start_url": baseURL + "/start",
	})
}

// HandleOnePassLogin exchanges a browser callback ticket for a local ANI session.
func (s *Server) HandleOnePassLogin(c *gin.Context) {
	if !s.onePassEnabled() {
		Fail(c, http.StatusServiceUnavailable, "1pass login is not configured")
		return
	}

	var req onePassLoginRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		Fail(c, http.StatusBadRequest, "ticket required")
		return
	}
	req.Ticket = strings.TrimSpace(req.Ticket)
	if req.Ticket == "" {
		Fail(c, http.StatusBadRequest, "ticket required")
		return
	}

	profile, err := s.exchangeOnePassTicket(req.Ticket)
	if err != nil {
		slog.Warn("1pass ticket exchange failed", "error", err)
		FailWithCode(c, http.StatusUnauthorized, ErrCodeAuthInvalid, "1pass login failed")
		return
	}
	if strings.TrimSpace(profile.OpenID) == "" {
		FailWithCode(c, http.StatusUnauthorized, ErrCodeAuthInvalid, "1pass response missing openid")
		return
	}
	if profile.SiteID != "" && profile.SiteID != s.Config.OnePassSiteID {
		FailWithCode(c, http.StatusUnauthorized, ErrCodeAuthInvalid, "1pass site mismatch")
		return
	}

	entity, err := s.findOrCreateOnePassEntity(c.Request.Context(), profile)
	if err != nil {
		slog.Error("failed to upsert 1pass entity", "error", err)
		Fail(c, http.StatusInternalServerError, "failed to create session")
		return
	}
	if entity.Status == "disabled" {
		FailWithCode(c, http.StatusForbidden, ErrCodePermDenied, "account is disabled")
		return
	}

	token, err := s.Auth.GenerateToken(entity.ID, entity.EntityType)
	if err != nil {
		Fail(c, http.StatusInternalServerError, "failed to generate token")
		return
	}

	auth.SetAuthCookie(c, token)
	s.attachEntityIdentity(c.Request.Context(), entity)
	OK(c, http.StatusOK, gin.H{
		"token":  token,
		"entity": entity,
	})
}

func (s *Server) exchangeOnePassTicket(ticket string) (*onePassUserInfo, error) {
	body, err := json.Marshal(map[string]string{
		"ticket": ticket,
		"format": "json",
	})
	if err != nil {
		return nil, err
	}

	ts := fmt.Sprintf("%d", time.Now().Unix())
	nonce, err := secureNonce()
	if err != nil {
		return nil, err
	}
	bodyHash := sha256.Sum256(body)
	canonical := fmt.Sprintf("%s\n%s\nPOST\n%s\n%x", ts, nonce, onePassTokenPath, bodyHash)
	signature := signOnePass(s.Config.OnePassSK, canonical)

	endpoint := strings.TrimRight(s.Config.OnePassBaseURL, "/") + onePassTokenPath
	httpReq, err := http.NewRequest(http.MethodPost, endpoint, bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("X-1Pass-AK", s.Config.OnePassAK)
	httpReq.Header.Set("X-1Pass-Ts", ts)
	httpReq.Header.Set("X-1Pass-Nonce", nonce)
	httpReq.Header.Set("X-1Pass-Sign", signature)

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(httpReq)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return nil, err
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("1pass token status %d: %s", resp.StatusCode, strings.TrimSpace(string(respBody)))
	}

	var profile onePassUserInfo
	if err := json.Unmarshal(respBody, &profile); err != nil {
		return nil, err
	}
	return &profile, nil
}

func signOnePass(secret, canonical string) string {
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write([]byte(canonical))
	return base64.RawURLEncoding.EncodeToString(mac.Sum(nil))
}

func secureNonce() (string, error) {
	var b [16]byte
	if _, err := rand.Read(b[:]); err != nil {
		return "", err
	}
	return uuid.Must(uuid.FromBytes(b[:])).String(), nil
}

func onePassEntityName(siteID, openID string) string {
	sum := sha256.Sum256([]byte(siteID + "\x00" + openID))
	return "1pass_" + hex.EncodeToString(sum[:])[:32]
}

func onePassDisplayName(profile *onePassUserInfo) string {
	if profile.Nickname != nil {
		if nickname := strings.TrimSpace(*profile.Nickname); nickname != "" {
			return nickname
		}
	}
	return "WeChat User"
}

func onePassMetadata(profile *onePassUserInfo) []byte {
	meta := map[string]any{
		"auth_provider": "1pass",
		"onepass": map[string]any{
			"site_id":   profile.SiteID,
			"openid":    profile.OpenID,
			"issued_at": profile.IssuedAt,
		},
	}
	if profile.UnionID != nil && strings.TrimSpace(*profile.UnionID) != "" {
		meta["onepass"].(map[string]any)["unionid"] = strings.TrimSpace(*profile.UnionID)
	}
	return mustJSONMetadata(meta)
}

func (s *Server) findOrCreateOnePassEntity(ctx context.Context, profile *onePassUserInfo) (*model.Entity, error) {
	name := onePassEntityName(s.Config.OnePassSiteID, profile.OpenID)
	if entity, err := s.Store.GetEntityByName(ctx, name, model.EntityUser); err == nil {
		changed := false
		displayName := onePassDisplayName(profile)
		if entity.DisplayName == "" || entity.DisplayName == "WeChat User" {
			entity.DisplayName = displayName
			changed = true
		}
		if profile.HeadimgURL != nil {
			if avatar := strings.TrimSpace(*profile.HeadimgURL); avatar != "" && entity.AvatarURL != avatar {
				entity.AvatarURL = avatar
				changed = true
			}
		}
		if len(entity.Metadata) == 0 {
			entity.Metadata = onePassMetadata(profile)
			changed = true
		}
		if changed {
			if err := s.Store.UpdateEntity(ctx, entity); err != nil {
				return nil, err
			}
		}
		return entity, nil
	}

	avatarURL := ""
	if profile.HeadimgURL != nil {
		avatarURL = strings.TrimSpace(*profile.HeadimgURL)
	}
	entity := &model.Entity{
		PublicID:    uuid.NewString(),
		EntityType:  model.EntityUser,
		Name:        name,
		DisplayName: onePassDisplayName(profile),
		AvatarURL:   avatarURL,
		Status:      "active",
		Metadata:    onePassMetadata(profile),
	}
	if err := s.Store.CreateEntity(ctx, entity); err != nil {
		// If another request created the user concurrently, load it.
		if existing, getErr := s.Store.GetEntityByName(ctx, name, model.EntityUser); getErr == nil {
			return existing, nil
		}
		return nil, err
	}
	return entity, nil
}
