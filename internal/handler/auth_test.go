package handler_test

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	"github.com/wzfukui/agent-native-im/internal/auth"
	"github.com/wzfukui/agent-native-im/internal/model"
)

func TestPing(t *testing.T) {
	resp := doJSON(t, "GET", "/api/v1/ping", nil, nil)
	assertStatus(t, resp, http.StatusOK)
}

func TestLogin(t *testing.T) {
	truncateAll(t)
	token := seedAdmin(t)
	if token == "" {
		t.Fatal("expected non-empty token")
	}
}

func TestLoginBadCredentials(t *testing.T) {
	truncateAll(t)
	seedAdmin(t)

	resp := doJSON(t, "POST", "/api/v1/auth/login", nil, map[string]string{
		"username": "testadmin",
		"password": "wrongpass",
	})
	assertStatus(t, resp, http.StatusUnauthorized)
}

func TestOnePassLoginCreatesAndReusesUser(t *testing.T) {
	truncateAll(t)

	const (
		siteID = "site_test"
		ak     = "ak_test"
		sk     = "sk_test"
		openID = "openid-test-user"
	)

	onePass := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/token" || r.Method != http.MethodPost {
			t.Fatalf("unexpected 1pass request: %s %s", r.Method, r.URL.Path)
		}
		if got := r.Header.Get("X-1Pass-AK"); got != ak {
			t.Fatalf("unexpected AK: %q", got)
		}
		body, err := io.ReadAll(r.Body)
		if err != nil {
			t.Fatalf("read 1pass body: %v", err)
		}
		var payload map[string]string
		if err := json.Unmarshal(body, &payload); err != nil {
			t.Fatalf("parse 1pass body: %v", err)
		}
		if payload["ticket"] == "" || payload["format"] != "json" {
			t.Fatalf("unexpected 1pass body: %s", body)
		}

		ts := r.Header.Get("X-1Pass-Ts")
		nonce := r.Header.Get("X-1Pass-Nonce")
		bodyHash := sha256.Sum256(body)
		canonical := fmt.Sprintf("%s\n%s\nPOST\n/token\n%x", ts, nonce, bodyHash)
		mac := hmac.New(sha256.New, []byte(sk))
		mac.Write([]byte(canonical))
		expectedSign := base64.RawURLEncoding.EncodeToString(mac.Sum(nil))
		if got := r.Header.Get("X-1Pass-Sign"); got != expectedSign {
			t.Fatalf("unexpected signature: got %q want %q", got, expectedSign)
		}

		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"site_id":"` + siteID + `","openid":"` + openID + `","unionid":null,"nickname":"Chris WeChat","headimgurl":"https://example.com/avatar.jpg","issued_at":"2026-05-07T12:00:00.000Z"}`))
	}))
	defer onePass.Close()

	oldConfig := *testServer.Config
	testServer.Config.OnePassSiteID = siteID
	testServer.Config.OnePassAK = ak
	testServer.Config.OnePassSK = sk
	testServer.Config.OnePassBaseURL = onePass.URL
	defer func() { *testServer.Config = oldConfig }()

	resp := doJSON(t, "GET", "/api/v1/auth/1pass/config", nil, nil)
	assertStatus(t, resp, http.StatusOK)
	cfg := parseOK(t, resp)
	if cfg["enabled"] != true || cfg["site_id"] != siteID || cfg["start_url"] != onePass.URL+"/start" {
		t.Fatalf("unexpected 1pass config: %#v", cfg)
	}

	resp = doJSON(t, "POST", "/api/v1/auth/1pass/login", nil, map[string]string{"ticket": "TK_first"})
	assertStatus(t, resp, http.StatusOK)
	first := parseOK(t, resp)
	firstEntity := first["entity"].(map[string]any)
	if first["token"] == "" || firstEntity["display_name"] != "Chris WeChat" {
		t.Fatalf("unexpected 1pass login payload: %#v", first)
	}
	firstToken := first["token"].(string)

	resp = doJSON(t, "GET", "/api/v1/me/auth-methods", ptr(firstToken), nil)
	assertStatus(t, resp, http.StatusOK)
	methods := parseOK(t, resp)
	if methods["has_password"] != false || methods["password_can_set"] != true {
		t.Fatalf("unexpected auth methods password state: %#v", methods)
	}
	identities, ok := methods["external_identities"].([]any)
	if !ok || len(identities) != 1 {
		t.Fatalf("expected one external identity, got %#v", methods["external_identities"])
	}
	identity := identities[0].(map[string]any)
	resp = doJSON(t, "DELETE", fmt.Sprintf("/api/v1/me/external-identities/%.0f", identity["id"]), ptr(firstToken), nil)
	assertStatus(t, resp, http.StatusConflict)

	resp = doJSON(t, "POST", "/api/v1/auth/1pass/login", nil, map[string]string{"ticket": "TK_second"})
	assertStatus(t, resp, http.StatusOK)
	second := parseOK(t, resp)
	secondEntity := second["entity"].(map[string]any)
	if firstEntity["id"] != secondEntity["id"] {
		t.Fatalf("expected repeated 1pass login to reuse entity, first=%v second=%v", firstEntity["id"], secondEntity["id"])
	}
}

func TestOnePassUserCanSetFirstPasswordAndUsername(t *testing.T) {
	truncateAll(t)

	const (
		siteID = "site_test_first_password"
		ak     = "ak_test"
		sk     = "sk_test"
		openID = "openid-first-password"
	)

	onePass := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"site_id":"` + siteID + `","openid":"` + openID + `","unionid":"union-first-password","nickname":"Mobile User","headimgurl":"","issued_at":"2026-05-10T12:00:00.000Z"}`))
	}))
	defer onePass.Close()

	oldConfig := *testServer.Config
	testServer.Config.OnePassSiteID = siteID
	testServer.Config.OnePassAK = ak
	testServer.Config.OnePassSK = sk
	testServer.Config.OnePassBaseURL = onePass.URL
	defer func() { *testServer.Config = oldConfig }()

	resp := doJSON(t, "POST", "/api/v1/auth/1pass/login", nil, map[string]string{"ticket": "TK_first"})
	assertStatus(t, resp, http.StatusOK)
	loginPayload := parseOK(t, resp)
	token := loginPayload["token"].(string)
	entity := loginPayload["entity"].(map[string]any)
	if name, _ := entity["name"].(string); len(name) < 7 || name[:6] != "1pass_" {
		t.Fatalf("expected legacy synthetic initial name, got %#v", entity["name"])
	}

	resp = doJSON(t, "PUT", "/api/v1/me/password", ptr(token), map[string]string{
		"username":     "mobileuser",
		"email":        "mobileuser@example.com",
		"new_password": "Mobilepass123",
	})
	assertStatus(t, resp, http.StatusOK)

	resp = doJSON(t, "POST", "/api/v1/auth/login", nil, map[string]string{
		"username": "mobileuser",
		"password": "Mobilepass123",
	})
	assertStatus(t, resp, http.StatusOK)

	resp = doJSON(t, "POST", "/api/v1/auth/1pass/login", nil, map[string]string{"ticket": "TK_second"})
	assertStatus(t, resp, http.StatusOK)
	second := parseOK(t, resp)
	secondEntity := second["entity"].(map[string]any)
	if secondEntity["name"] != "mobileuser" {
		t.Fatalf("expected 1pass login to reuse renamed entity, got %#v", secondEntity)
	}

	secondToken := second["token"].(string)
	resp = doJSON(t, "GET", "/api/v1/me/auth-methods", ptr(secondToken), nil)
	assertStatus(t, resp, http.StatusOK)
	methods := parseOK(t, resp)
	identities, ok := methods["external_identities"].([]any)
	if !ok || len(identities) != 1 {
		t.Fatalf("expected one external identity after password setup, got %#v", methods["external_identities"])
	}
	identity := identities[0].(map[string]any)
	resp = doJSON(t, "DELETE", fmt.Sprintf("/api/v1/me/external-identities/%.0f", identity["id"]), ptr(secondToken), nil)
	assertStatus(t, resp, http.StatusOK)
}

func TestMeWithJWT(t *testing.T) {
	truncateAll(t)
	token := seedAdmin(t)

	resp := doJSON(t, "GET", "/api/v1/me", ptr(token), nil)
	assertStatus(t, resp, http.StatusOK)

	data := parseOK(t, resp)
	entity, ok := data["name"]
	if !ok || entity != "testadmin" {
		t.Fatalf("expected name=testadmin, got %v", entity)
	}
	publicID, _ := data["public_id"].(string)
	if _, err := uuid.Parse(publicID); err != nil {
		t.Fatalf("expected valid public_id UUID, got %q", publicID)
	}
}

func TestMeWithoutAuth(t *testing.T) {
	resp := doJSON(t, "GET", "/api/v1/me", nil, nil)
	assertStatus(t, resp, http.StatusUnauthorized)
}

func TestChangePassword(t *testing.T) {
	truncateAll(t)
	token := seedAdmin(t)

	// Change password
	resp := doJSON(t, "PUT", "/api/v1/me/password", ptr(token), map[string]string{
		"old_password": "testpass",
		"new_password": "Newpass123",
	})
	assertStatus(t, resp, http.StatusOK)

	// Old password should no longer work
	resp = doJSON(t, "POST", "/api/v1/auth/login", nil, map[string]string{
		"username": "testadmin",
		"password": "testpass",
	})
	assertStatus(t, resp, http.StatusUnauthorized)

	// New password should work
	resp = doJSON(t, "POST", "/api/v1/auth/login", nil, map[string]string{
		"username": "testadmin",
		"password": "Newpass123",
	})
	assertStatus(t, resp, http.StatusOK)
}

func TestChangePasswordWrongOld(t *testing.T) {
	truncateAll(t)
	token := seedAdmin(t)

	resp := doJSON(t, "PUT", "/api/v1/me/password", ptr(token), map[string]string{
		"old_password": "wrong",
		"new_password": "Newpass123",
	})
	assertStatus(t, resp, http.StatusUnauthorized)
}

func TestCreateUser(t *testing.T) {
	truncateAll(t)
	token := seedAdmin(t)

	// Create a new user
	resp := doJSON(t, "POST", "/api/v1/admin/users", ptr(token), map[string]string{
		"username":     "newuser",
		"password":     "Userpass123",
		"display_name": "New User",
	})
	assertStatus(t, resp, http.StatusCreated)

	data := parseOK(t, resp)
	if data["name"] != "newuser" {
		t.Fatalf("expected name=newuser, got %v", data["name"])
	}
	publicID, _ := data["public_id"].(string)
	if _, err := uuid.Parse(publicID); err != nil {
		t.Fatalf("expected valid public_id UUID, got %q", publicID)
	}

	// New user should be able to login
	resp = doJSON(t, "POST", "/api/v1/auth/login", nil, map[string]string{
		"username": "newuser",
		"password": "Userpass123",
	})
	assertStatus(t, resp, http.StatusOK)
}

func TestRegisterReturnsPublicID(t *testing.T) {
	truncateAll(t)

	resp := doJSON(t, "POST", "/api/v1/auth/register", nil, map[string]string{
		"username":     "publicid-user",
		"password":     "Publicid1",
		"display_name": "Public ID User",
	})
	assertStatus(t, resp, http.StatusCreated)

	data := parseOK(t, resp)
	entity, ok := data["entity"].(map[string]any)
	if !ok {
		t.Fatalf("expected entity payload, got %T", data["entity"])
	}
	publicID, _ := entity["public_id"].(string)
	if _, err := uuid.Parse(publicID); err != nil {
		t.Fatalf("expected valid public_id UUID, got %q", publicID)
	}
}

func TestRegisterAcceptsComplexPasswordWithoutWeakWordFiltering(t *testing.T) {
	truncateAll(t)

	resp := doJSON(t, "POST", "/api/v1/auth/register", nil, map[string]string{
		"username": "admin-token-user",
		"password": "Admin@123456",
	})
	assertStatus(t, resp, http.StatusCreated)
}

func TestCreateUserShortPassword(t *testing.T) {
	truncateAll(t)
	token := seedAdmin(t)

	resp := doJSON(t, "POST", "/api/v1/admin/users", ptr(token), map[string]string{
		"username": "shortpw",
		"password": "12345",
	})
	assertStatus(t, resp, http.StatusBadRequest)
}

func TestRefreshToken(t *testing.T) {
	truncateAll(t)
	token := seedAdmin(t)

	// Refresh token
	resp := doJSON(t, "POST", "/api/v1/auth/refresh", ptr(token), nil)
	assertStatus(t, resp, http.StatusOK)
	data := parseOK(t, resp)
	newToken, ok := data["token"].(string)
	if !ok || newToken == "" {
		t.Fatal("expected new token from refresh")
	}

	// New token should work
	resp = doJSON(t, "GET", "/api/v1/me", ptr(newToken), nil)
	assertStatus(t, resp, http.StatusOK)
}

func TestRefreshTokenWithRecentlyExpiredJWT(t *testing.T) {
	truncateAll(t)
	validToken := seedAdmin(t)
	claims, err := auth.ParseToken("test-secret", validToken)
	if err != nil {
		t.Fatalf("parse token: %v", err)
	}

	expiredTokenObj := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
		"entity_id":   float64(claims.EntityID),
		"entity_type": string(model.EntityUser),
		"exp":         time.Now().Add(-2 * time.Hour).Unix(),
		"iat":         time.Now().Add(-26 * time.Hour).Unix(),
	})
	expiredToken, err := expiredTokenObj.SignedString([]byte("test-secret"))
	if err != nil {
		t.Fatalf("failed to sign expired token: %v", err)
	}

	resp := doJSON(t, "POST", "/api/v1/auth/refresh", &expiredToken, nil)
	assertStatus(t, resp, http.StatusOK)
	data := parseOK(t, resp)
	newToken, ok := data["token"].(string)
	if !ok || newToken == "" {
		t.Fatal("expected new token from refresh with expired JWT")
	}
}

func TestRefreshTokenRejectsTooOldExpiredJWT(t *testing.T) {
	truncateAll(t)
	validToken := seedAdmin(t)
	claims, err := auth.ParseToken("test-secret", validToken)
	if err != nil {
		t.Fatalf("parse token: %v", err)
	}

	oldExpiredTokenObj := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
		"entity_id":   float64(claims.EntityID),
		"entity_type": string(model.EntityUser),
		"exp":         time.Now().Add(-8 * 24 * time.Hour).Unix(),
		"iat":         time.Now().Add(-9 * 24 * time.Hour).Unix(),
	})
	oldExpiredToken, err := oldExpiredTokenObj.SignedString([]byte("test-secret"))
	if err != nil {
		t.Fatalf("failed to sign old expired token: %v", err)
	}

	resp := doJSON(t, "POST", "/api/v1/auth/refresh", &oldExpiredToken, nil)
	assertStatus(t, resp, http.StatusUnauthorized)
}

func TestRefreshTokenRejectsBotEntity(t *testing.T) {
	truncateAll(t)
	token := seedAdmin(t)

	resp := doJSON(t, "POST", "/api/v1/entities", ptr(token), map[string]string{"name": "refresh-bot"})
	assertStatus(t, resp, http.StatusCreated)
	data := parseOK(t, resp)
	// New flow: creation returns api_key (aim_ prefix) directly
	apiKey, _ := data["api_key"].(string)
	if apiKey == "" {
		t.Fatal("expected api_key")
	}

	// Bot API keys should not be refreshable via JWT refresh endpoint
	resp = doJSON(t, "POST", "/api/v1/auth/refresh", &apiKey, nil)
	assertStatus(t, resp, http.StatusForbidden)
}

func TestUpdateProfile(t *testing.T) {
	truncateAll(t)
	token := seedAdmin(t)

	// Update display name
	resp := doJSON(t, "PUT", "/api/v1/me", ptr(token), map[string]string{
		"display_name": "Chris Admin",
	})
	assertStatus(t, resp, http.StatusOK)
	data := parseOK(t, resp)
	if data["display_name"] != "Chris Admin" {
		t.Fatalf("expected display_name=Chris Admin, got %v", data["display_name"])
	}

	// Update avatar
	resp = doJSON(t, "PUT", "/api/v1/me", ptr(token), map[string]string{
		"avatar_url": "https://example.com/avatar.png",
	})
	assertStatus(t, resp, http.StatusOK)
	data = parseOK(t, resp)
	if data["avatar_url"] != "https://example.com/avatar.png" {
		t.Fatalf("expected avatar_url updated, got %v", data["avatar_url"])
	}

	// Verify via /me
	resp = doJSON(t, "GET", "/api/v1/me", ptr(token), nil)
	assertStatus(t, resp, http.StatusOK)
	data = parseOK(t, resp)
	if data["display_name"] != "Chris Admin" {
		t.Fatalf("expected display_name persisted, got %v", data["display_name"])
	}
}

func TestUpdateProfileNormalizesStableAvatarRoutes(t *testing.T) {
	truncateAll(t)
	token := seedAdmin(t)

	resp := doJSON(t, "PUT", "/api/v1/me", ptr(token), map[string]string{
		"avatar_url": "/avatar-files/profile.png?v=1",
	})
	assertStatus(t, resp, http.StatusOK)
	data := parseOK(t, resp)
	if data["avatar_url"] != "/files/profile.png" {
		t.Fatalf("expected normalized avatar_url=/files/profile.png, got %v", data["avatar_url"])
	}

	resp = doJSON(t, "PUT", "/api/v1/me", ptr(token), map[string]string{
		"avatar_url": "/avatars/profile.png",
	})
	assertStatus(t, resp, http.StatusOK)
	data = parseOK(t, resp)
	if data["avatar_url"] != "/files/profile.png" {
		t.Fatalf("expected legacy /avatars route normalized to /files/profile.png, got %v", data["avatar_url"])
	}
}

func TestUpdateProfileEmpty(t *testing.T) {
	truncateAll(t)
	token := seedAdmin(t)

	// Empty update should fail
	resp := doJSON(t, "PUT", "/api/v1/me", ptr(token), map[string]interface{}{})
	assertStatus(t, resp, http.StatusBadRequest)
}

func TestCreateUserNonAdmin(t *testing.T) {
	truncateAll(t)
	token := seedAdmin(t)

	// Create a regular user
	resp := doJSON(t, "POST", "/api/v1/admin/users", ptr(token), map[string]string{
		"username": "regular",
		"password": "Regular123",
	})
	assertStatus(t, resp, http.StatusCreated)

	regularToken := login(t, "regular", "Regular123")

	// Regular user tries to create a user — should fail (admin only)
	resp = doJSON(t, "POST", "/api/v1/admin/users", ptr(regularToken), map[string]string{
		"username": "another",
		"password": "Another123",
	})
	assertStatus(t, resp, http.StatusForbidden)
}

func TestLoginDisabledUserForbidden(t *testing.T) {
	truncateAll(t)
	adminToken := seedAdmin(t)

	resp := doJSON(t, "POST", "/api/v1/admin/users", ptr(adminToken), map[string]string{
		"username": "disabled-user",
		"password": "Disabled123",
	})
	assertStatus(t, resp, http.StatusCreated)
	data := parseOK(t, resp)
	userID := int64(data["id"].(float64))

	resp = doJSON(t, "PUT", fmt.Sprintf("/api/v1/admin/users/%d", userID), ptr(adminToken), map[string]string{
		"status": "disabled",
	})
	assertStatus(t, resp, http.StatusOK)

	resp = doJSON(t, "POST", "/api/v1/auth/login", nil, map[string]string{
		"username": "disabled-user",
		"password": "Disabled123",
	})
	assertStatus(t, resp, http.StatusForbidden)
}

func TestDisabledUserTokenRejectedByMeAndRefresh(t *testing.T) {
	truncateAll(t)
	adminToken := seedAdmin(t)

	resp := doJSON(t, "POST", "/api/v1/admin/users", ptr(adminToken), map[string]string{
		"username": "disabled-after-login",
		"password": "Disabled123",
	})
	assertStatus(t, resp, http.StatusCreated)
	data := parseOK(t, resp)
	userID := int64(data["id"].(float64))

	userToken := login(t, "disabled-after-login", "Disabled123")

	resp = doJSON(t, "PUT", fmt.Sprintf("/api/v1/admin/users/%d", userID), ptr(adminToken), map[string]string{
		"status": "disabled",
	})
	assertStatus(t, resp, http.StatusOK)

	resp = doJSON(t, "GET", "/api/v1/me", ptr(userToken), nil)
	assertStatus(t, resp, http.StatusForbidden)

	resp = doJSON(t, "POST", "/api/v1/auth/refresh", ptr(userToken), nil)
	assertStatus(t, resp, http.StatusForbidden)
}
