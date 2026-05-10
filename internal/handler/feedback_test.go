package handler_test

import (
	"fmt"
	"net/http"
	"testing"
)

func TestFeedbackUserAndAdminFlow(t *testing.T) {
	truncateAll(t)
	adminToken := seedAdmin(t)

	resp := doJSON(t, "POST", "/api/v1/auth/register", nil, map[string]string{
		"username": "feedback-user",
		"password": "Userpass123",
	})
	assertStatus(t, resp, http.StatusCreated)
	userPayload := parseOK(t, resp)
	userToken := userPayload["token"].(string)

	resp = doJSON(t, "POST", "/api/v1/feedback", ptr(userToken), map[string]any{
		"type":        "bug",
		"severity":    "high",
		"title":       "Login spinner never stops",
		"description": "The browser keeps loading after sign-in.",
		"contact":     "user@example.com",
		"attachments": []map[string]any{
			{"type": "image", "url": "/files/demo.png", "filename": "demo.png", "mime_type": "image/png", "size": 123},
		},
	})
	assertStatus(t, resp, http.StatusCreated)
	created := parseOK(t, resp)
	feedbackID := int(created["id"].(float64))
	if created["status"] != "open" || created["type"] != "bug" || created["severity"] != "high" {
		t.Fatalf("unexpected feedback payload: %#v", created)
	}

	resp = doJSON(t, "GET", "/api/v1/feedback", ptr(userToken), nil)
	assertStatus(t, resp, http.StatusOK)
	userList := parseOK(t, resp)
	if userList["admin"] != false || int(userList["total"].(float64)) != 1 {
		t.Fatalf("unexpected user feedback list: %#v", userList)
	}

	resp = doJSON(t, "POST", fmt.Sprintf("/api/v1/feedback/%d/comments", feedbackID), ptr(adminToken), map[string]string{
		"body":       "Internal triage note",
		"visibility": "internal",
	})
	assertStatus(t, resp, http.StatusCreated)

	resp = doJSON(t, "GET", fmt.Sprintf("/api/v1/feedback/%d", feedbackID), ptr(userToken), nil)
	assertStatus(t, resp, http.StatusOK)
	userDetail := parseOK(t, resp)
	if comments := userDetail["comments"].([]any); len(comments) != 0 {
		t.Fatalf("normal user should not see internal comments: %#v", comments)
	}

	resp = doJSON(t, "GET", fmt.Sprintf("/api/v1/feedback/%d", feedbackID), ptr(adminToken), nil)
	assertStatus(t, resp, http.StatusOK)
	adminDetail := parseOK(t, resp)
	if comments := adminDetail["comments"].([]any); len(comments) != 1 {
		t.Fatalf("admin should see internal comments: %#v", comments)
	}

	resp = doJSON(t, "PATCH", fmt.Sprintf("/api/v1/admin/feedback/%d", feedbackID), ptr(adminToken), map[string]string{
		"status":   "triaged",
		"priority": "urgent",
	})
	assertStatus(t, resp, http.StatusOK)
	updated := parseOK(t, resp)
	if updated["status"] != "triaged" || updated["priority"] != "urgent" {
		t.Fatalf("unexpected admin update payload: %#v", updated)
	}
}

func TestFeedbackAccessIsScopedToSubmitter(t *testing.T) {
	truncateAll(t)

	resp := doJSON(t, "POST", "/api/v1/auth/register", nil, map[string]string{
		"username": "feedback-owner",
		"password": "Userpass123",
	})
	assertStatus(t, resp, http.StatusCreated)
	ownerToken := parseOK(t, resp)["token"].(string)

	resp = doJSON(t, "POST", "/api/v1/auth/register", nil, map[string]string{
		"username": "feedback-other",
		"password": "Userpass123",
	})
	assertStatus(t, resp, http.StatusCreated)
	otherToken := parseOK(t, resp)["token"].(string)

	resp = doJSON(t, "POST", "/api/v1/feedback", ptr(ownerToken), map[string]string{
		"title":       "Private issue",
		"description": "Only the owner should see this.",
	})
	assertStatus(t, resp, http.StatusCreated)
	feedbackID := int(parseOK(t, resp)["id"].(float64))

	resp = doJSON(t, "GET", "/api/v1/feedback", ptr(otherToken), nil)
	assertStatus(t, resp, http.StatusOK)
	list := parseOK(t, resp)
	if int(list["total"].(float64)) != 0 {
		t.Fatalf("other user should not list owner feedback: %#v", list)
	}

	resp = doJSON(t, "GET", fmt.Sprintf("/api/v1/feedback/%d", feedbackID), ptr(otherToken), nil)
	assertStatus(t, resp, http.StatusForbidden)
}
