package handler_test

import (
	"context"
	"fmt"
	"net/http"
	"testing"

	"github.com/google/uuid"
	"github.com/wzfukui/agent-native-im/internal/model"
)

func seedRelease(t *testing.T, version string) int64 {
	t.Helper()
	release := &model.Release{
		PublicID:  uuid.NewString(),
		Version:   version,
		Component: "platform",
		Platform:  "all",
		Channel:   "production",
		Title:     "ANI " + version,
		Summary:   "Release summary",
		Sections: []model.ReleaseSection{{
			Kind:  "fixed",
			Title: "Fixes",
			Items: []string{"A user-facing fix"},
		}},
		RequiredActions: []model.ReleaseAction{{
			Component: "openclaw_plugin",
			Title:     "Upgrade plugin",
			Body:      "Run the installer update.",
		}},
		KnownIssues: []string{},
	}
	if _, err := testStore.DB.NewInsert().Model(release).Exec(context.Background()); err != nil {
		t.Fatalf("seed release: %v", err)
	}
	return release.ID
}

func TestReleaseListLatestAndRead(t *testing.T) {
	truncateAll(t)
	seedRelease(t, "2026.5.14")

	resp := doJSON(t, "POST", "/api/v1/auth/register", nil, map[string]string{
		"username": "release-reader",
		"password": "Userpass123",
	})
	assertStatus(t, resp, http.StatusCreated)
	token := parseOK(t, resp)["token"].(string)

	resp = doJSON(t, "GET", "/api/v1/releases", ptr(token), nil)
	assertStatus(t, resp, http.StatusOK)
	list := parseOK(t, resp)
	if int(list["total"].(float64)) != 1 || int(list["unread_count"].(float64)) != 1 {
		t.Fatalf("unexpected release list: %#v", list)
	}
	items := list["items"].([]any)
	releaseID := int(items[0].(map[string]any)["id"].(float64))

	resp = doJSON(t, "GET", "/api/v1/releases/latest", ptr(token), nil)
	assertStatus(t, resp, http.StatusOK)
	latest := parseOK(t, resp)
	if latest["release"].(map[string]any)["version"] != "2026.5.14" {
		t.Fatalf("unexpected latest release: %#v", latest)
	}

	resp = doJSON(t, "POST", fmt.Sprintf("/api/v1/releases/%d/read", releaseID), ptr(token), nil)
	assertStatus(t, resp, http.StatusOK)

	resp = doJSON(t, "GET", "/api/v1/releases", ptr(token), nil)
	assertStatus(t, resp, http.StatusOK)
	readList := parseOK(t, resp)
	if int(readList["unread_count"].(float64)) != 0 {
		t.Fatalf("expected release to be marked read: %#v", readList)
	}
}

func TestFeedbackCanLinkRelease(t *testing.T) {
	truncateAll(t)
	adminToken := seedAdmin(t)
	releaseID := seedRelease(t, "2026.5.14")

	resp := doJSON(t, "POST", "/api/v1/auth/register", nil, map[string]string{
		"username": "feedback-release-owner",
		"password": "Userpass123",
	})
	assertStatus(t, resp, http.StatusCreated)
	userToken := parseOK(t, resp)["token"].(string)

	resp = doJSON(t, "POST", "/api/v1/feedback", ptr(userToken), map[string]string{
		"title":       "Push notification failed",
		"description": "iOS push did not arrive.",
	})
	assertStatus(t, resp, http.StatusCreated)
	feedbackID := int(parseOK(t, resp)["id"].(float64))

	resp = doJSON(t, "PATCH", fmt.Sprintf("/api/v1/admin/feedback/%d", feedbackID), ptr(adminToken), map[string]any{
		"status":               "resolved",
		"fixed_in_release_ids": []int64{releaseID},
	})
	assertStatus(t, resp, http.StatusOK)

	resp = doJSON(t, "GET", fmt.Sprintf("/api/v1/feedback/%d", feedbackID), ptr(userToken), nil)
	assertStatus(t, resp, http.StatusOK)
	detail := parseOK(t, resp)
	links := detail["releases"].([]any)
	if len(links) != 1 || links[0].(map[string]any)["link_type"] != "fixed" {
		t.Fatalf("expected fixed release link: %#v", detail)
	}
}
