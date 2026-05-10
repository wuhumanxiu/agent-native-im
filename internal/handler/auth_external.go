package handler

import (
	"net/http"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/wzfukui/agent-native-im/internal/auth"
	"github.com/wzfukui/agent-native-im/internal/model"
)

// HandleAuthMethods returns first-party and external login methods for the current account.
func (s *Server) HandleAuthMethods(c *gin.Context) {
	if auth.GetEntityType(c) != model.EntityUser {
		FailWithCode(c, http.StatusForbidden, ErrCodePermDenied, "only users have auth methods")
		return
	}
	entityID := auth.GetEntityID(c)
	ctx := c.Request.Context()

	hasPassword, err := s.hasPasswordCredential(ctx, entityID)
	if err != nil {
		Fail(c, http.StatusInternalServerError, "failed to load password status")
		return
	}
	identities, err := s.Store.ListExternalIdentitiesByEntity(ctx, entityID)
	if err != nil {
		Fail(c, http.StatusInternalServerError, "failed to load external identities")
		return
	}

	OK(c, http.StatusOK, gin.H{
		"has_password":        hasPassword,
		"password_can_set":    !hasPassword,
		"external_identities": identities,
	})
}

// HandleLinkOnePass binds a 1Pass callback ticket to the current ANI account.
func (s *Server) HandleLinkOnePass(c *gin.Context) {
	if auth.GetEntityType(c) != model.EntityUser {
		FailWithCode(c, http.StatusForbidden, ErrCodePermDenied, "only users can link 1Pass")
		return
	}
	if !s.onePassEnabled() {
		Fail(c, http.StatusServiceUnavailable, "1Pass login is not configured")
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
		FailWithCode(c, http.StatusUnauthorized, ErrCodeAuthInvalid, "1Pass link failed")
		return
	}
	if strings.TrimSpace(profile.OpenID) == "" {
		FailWithCode(c, http.StatusUnauthorized, ErrCodeAuthInvalid, "1Pass response missing openid")
		return
	}
	if profile.SiteID != "" && profile.SiteID != s.Config.OnePassSiteID {
		FailWithCode(c, http.StatusUnauthorized, ErrCodeAuthInvalid, "1Pass site mismatch")
		return
	}

	ctx := c.Request.Context()
	entityID := auth.GetEntityID(c)
	siteID := strings.TrimSpace(profile.SiteID)
	if siteID == "" {
		siteID = s.Config.OnePassSiteID
	}
	providerSubject := onePassProviderSubject(siteID, profile.OpenID)
	if existing, err := s.Store.GetExternalIdentityByProviderSubject(ctx, "1pass", providerSubject); err == nil && existing != nil && existing.EntityID != entityID {
		FailWithCode(c, http.StatusConflict, ErrCodeConflict, "1Pass identity is already linked to another account")
		return
	}

	identity, err := s.upsertOnePassExternalIdentity(ctx, entityID, profile)
	if err != nil {
		Fail(c, http.StatusInternalServerError, "failed to link 1Pass identity")
		return
	}
	OK(c, http.StatusOK, identity)
}

// HandleUnlinkExternalIdentity removes a bound external identity when another auth method remains.
func (s *Server) HandleUnlinkExternalIdentity(c *gin.Context) {
	if auth.GetEntityType(c) != model.EntityUser {
		FailWithCode(c, http.StatusForbidden, ErrCodePermDenied, "only users can unlink external identities")
		return
	}
	identityID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || identityID <= 0 {
		Fail(c, http.StatusBadRequest, "invalid external identity id")
		return
	}

	ctx := c.Request.Context()
	entityID := auth.GetEntityID(c)
	identity, err := s.Store.GetExternalIdentityByID(ctx, identityID)
	if err != nil {
		FailWithCode(c, http.StatusNotFound, ErrCodeNotFound, "external identity not found")
		return
	}
	if identity.EntityID != entityID {
		FailWithCode(c, http.StatusForbidden, ErrCodePermDenied, "external identity does not belong to current account")
		return
	}

	hasPassword, err := s.hasPasswordCredential(ctx, entityID)
	if err != nil {
		Fail(c, http.StatusInternalServerError, "failed to load password status")
		return
	}
	identities, err := s.Store.ListExternalIdentitiesByEntity(ctx, entityID)
	if err != nil {
		Fail(c, http.StatusInternalServerError, "failed to load external identities")
		return
	}
	if !hasPassword && len(identities) <= 1 {
		FailWithCode(c, http.StatusConflict, ErrCodeStateBadTransition, "cannot remove the last auth method")
		return
	}

	if err := s.Store.DeleteExternalIdentity(ctx, identityID); err != nil {
		Fail(c, http.StatusInternalServerError, "failed to unlink external identity")
		return
	}
	OK(c, http.StatusOK, "external identity unlinked")
}
