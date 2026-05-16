package handler_test

import (
	"context"
	"encoding/json"
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
		TitleI18N: map[string]string{
			"en":    "ANI " + version,
			"zh-CN": "ANI " + version + " 更新",
		},
		SummaryI18N: map[string]string{
			"en":    "Release summary",
			"zh-CN": "版本摘要",
		},
		SectionsI18N:    map[string][]model.ReleaseSection{},
		ActionsI18N:     map[string][]model.ReleaseAction{},
		KnownIssuesI18N: map[string][]string{},
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
	releases := list["releases"].([]any)
	if len(releases) != 1 || releases[0].(map[string]any)["is_read"].(bool) {
		t.Fatalf("expected unread release list: %#v", list)
	}
	titleI18N := releases[0].(map[string]any)["title_i18n"].(map[string]any)
	if titleI18N["zh-CN"] != "ANI 2026.5.14 更新" {
		t.Fatalf("expected localized release title: %#v", releases[0])
	}
	releaseID := int(releases[0].(map[string]any)["id"].(float64))

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
	readReleases := readList["releases"].([]any)
	if len(readReleases) != 1 || !readReleases[0].(map[string]any)["is_read"].(bool) {
		t.Fatalf("expected read release marker: %#v", readList)
	}
}

func TestCreateReleaseCreatesUnreadNotificationsForUsers(t *testing.T) {
	truncateAll(t)

	for _, username := range []string{"release-notify-a", "release-notify-b"} {
		resp := doJSON(t, "POST", "/api/v1/auth/register", nil, map[string]string{
			"username": username,
			"password": "Userpass123",
		})
		assertStatus(t, resp, http.StatusCreated)
	}

	release := &model.Release{
		PublicID:  uuid.NewString(),
		Version:   "2026.5.16",
		Component: "platform",
		Platform:  "all",
		Channel:   "production",
		Title:     "Release notification test",
		Summary:   "Users should see this release in the inbox.",
		Sections: []model.ReleaseSection{{
			Kind:  "new",
			Title: "Release notifications",
			Items: []string{"Users receive unread system notifications."},
		}},
		RequiredActions: []model.ReleaseAction{},
		KnownIssues:     []string{},
		TitleI18N: map[string]string{
			"en":    "Release notification test",
			"zh-CN": "版本通知测试",
		},
		SummaryI18N: map[string]string{
			"en":    "Users should see this release in the inbox.",
			"zh-CN": "用户应在收件箱看到此版本通知。",
		},
		SectionsI18N:    map[string][]model.ReleaseSection{},
		ActionsI18N:     map[string][]model.ReleaseAction{},
		KnownIssuesI18N: map[string][]string{},
	}
	if err := testStore.CreateRelease(context.Background(), release); err != nil {
		t.Fatalf("create release: %v", err)
	}

	count, err := testStore.DB.NewSelect().
		Model((*model.Notification)(nil)).
		Where("kind = ?", "release.published").
		Where("status = ?", model.NotificationUnread).
		Where("data->>'release_id' = ?", fmt.Sprint(release.ID)).
		Count(context.Background())
	if err != nil {
		t.Fatalf("count release notifications: %v", err)
	}
	if count != 2 {
		t.Fatalf("expected release notification for each user, got %d", count)
	}

	var notifications []*model.Notification
	if err := testStore.DB.NewSelect().
		Model(&notifications).
		Where("kind = ?", "release.published").
		Where("data->>'release_id' = ?", fmt.Sprint(release.ID)).
		Scan(context.Background()); err != nil {
		t.Fatalf("list release notifications: %v", err)
	}
	for _, notification := range notifications {
		if notification.Title != "ANI update 2026.5.16" {
			t.Fatalf("unexpected notification title: %#v", notification)
		}
		var payload map[string]any
		if err := json.Unmarshal(notification.Data, &payload); err != nil {
			t.Fatalf("decode notification payload: %v", err)
		}
		if payload["path"] != "/settings/releases" {
			t.Fatalf("expected release center path in notification payload: %#v", payload)
		}
		titleI18N := payload["title_i18n"].(map[string]any)
		if titleI18N["zh-CN"] != "版本通知测试" {
			t.Fatalf("expected localized notification title: %#v", payload)
		}
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
