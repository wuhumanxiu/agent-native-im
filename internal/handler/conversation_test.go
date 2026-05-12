package handler_test

import (
	"fmt"
	"net/http"
	"testing"

	"github.com/google/uuid"
)

func TestCreateConversation(t *testing.T) {
	truncateAll(t)
	token := seedAdmin(t)

	botResp := doJSON(t, "POST", "/api/v1/entities", ptr(token), map[string]string{"name": "test-chat-bot"})
	assertStatus(t, botResp, http.StatusCreated)
	botData := parseOK(t, botResp)
	botID := botData["entity"].(map[string]interface{})["id"].(float64)

	resp := doJSON(t, "POST", "/api/v1/conversations", ptr(token), map[string]interface{}{
		"title":           "Test Chat",
		"conv_type":       "direct",
		"participant_ids": []float64{botID},
	})
	assertStatus(t, resp, http.StatusCreated)

	data := parseOK(t, resp)
	if data["title"] != "Test Chat" {
		t.Fatalf("expected title=Test Chat, got %v", data["title"])
	}
	meta, _ := data["metadata"].(map[string]interface{})
	publicID, _ := meta["public_id"].(string)
	if _, err := uuid.Parse(publicID); err != nil {
		t.Fatalf("expected valid metadata.public_id UUID, got %q", publicID)
	}
}

func TestCreateDirectConversationReusesExistingConversation(t *testing.T) {
	truncateAll(t)
	adminToken := seedAdmin(t)
	meResp := doJSON(t, "GET", "/api/v1/me", ptr(adminToken), nil)
	assertStatus(t, meResp, http.StatusOK)
	adminID := int64(parseOK(t, meResp)["id"].(float64))

	userResp := doJSON(t, "POST", "/api/v1/admin/users", ptr(adminToken), map[string]string{
		"username": "direct-peer",
		"password": "Peerpass1",
	})
	assertStatus(t, userResp, http.StatusCreated)
	peerID := int64(parseOK(t, userResp)["id"].(float64))
	peerToken := login(t, "direct-peer", "Peerpass1")

	reqResp := doJSON(t, "POST", "/api/v1/friends/requests", ptr(adminToken), map[string]any{
		"target_entity_id": peerID,
	})
	assertStatus(t, reqResp, http.StatusCreated)
	reqID := int(parseOK(t, reqResp)["id"].(float64))

	acceptResp := doJSON(t, "POST", fmt.Sprintf("/api/v1/friends/requests/%d/accept", reqID), ptr(peerToken), nil)
	assertStatus(t, acceptResp, http.StatusOK)

	firstResp := doJSON(t, "POST", "/api/v1/conversations", ptr(adminToken), map[string]any{
		"title":           "Admin -> Peer",
		"conv_type":       "direct",
		"participant_ids": []int64{peerID},
	})
	assertStatus(t, firstResp, http.StatusCreated)
	first := parseOK(t, firstResp)

	secondResp := doJSON(t, "POST", "/api/v1/conversations", ptr(peerToken), map[string]any{
		"title":           "Peer -> Admin",
		"conv_type":       "direct",
		"participant_ids": []int64{adminID},
	})
	assertStatus(t, secondResp, http.StatusOK)
	second := parseOK(t, secondResp)

	if first["id"] != second["id"] {
		t.Fatalf("expected direct conversation reuse, got %v and %v", first["id"], second["id"])
	}
}

func TestCreateDirectConversationAsOwnedBot(t *testing.T) {
	truncateAll(t)
	adminToken := seedAdmin(t)

	botResp := doJSON(t, "POST", "/api/v1/entities", ptr(adminToken), map[string]string{"name": "owned-agent"})
	assertStatus(t, botResp, http.StatusCreated)
	botEntity := parseOK(t, botResp)["entity"].(map[string]interface{})
	botID := int64(botEntity["id"].(float64))

	userResp := doJSON(t, "POST", "/api/v1/admin/users", ptr(adminToken), map[string]string{
		"username": "acting-peer",
		"password": "Peerpass1",
	})
	assertStatus(t, userResp, http.StatusCreated)
	peerID := int64(parseOK(t, userResp)["id"].(float64))
	peerToken := login(t, "acting-peer", "Peerpass1")

	reqResp := doJSON(t, "POST", "/api/v1/friends/requests", ptr(adminToken), map[string]any{
		"source_entity_id": botID,
		"target_entity_id": peerID,
	})
	assertStatus(t, reqResp, http.StatusCreated)
	reqID := int(parseOK(t, reqResp)["id"].(float64))

	acceptResp := doJSON(t, "POST", fmt.Sprintf("/api/v1/friends/requests/%d/accept", reqID), ptr(peerToken), nil)
	assertStatus(t, acceptResp, http.StatusOK)

	convResp := doJSON(t, "POST", "/api/v1/conversations", ptr(adminToken), map[string]any{
		"source_entity_id": botID,
		"title":            "Bot -> Peer",
		"conv_type":        "direct",
		"participant_ids":  []int64{peerID},
	})
	assertStatus(t, convResp, http.StatusCreated)
	conv := parseOK(t, convResp)
	if conv["conv_type"] != "direct" {
		t.Fatalf("expected direct conversation, got %v", conv["conv_type"])
	}
}

func TestCreateConversationAcceptsPublicIDs(t *testing.T) {
	truncateAll(t)
	adminToken := seedAdmin(t)

	botResp := doJSON(t, "POST", "/api/v1/entities", ptr(adminToken), map[string]string{"name": "public-id-source-bot"})
	assertStatus(t, botResp, http.StatusCreated)
	botEntity := parseOK(t, botResp)["entity"].(map[string]interface{})
	botPublicID := botEntity["public_id"].(string)

	userResp := doJSON(t, "POST", "/api/v1/admin/users", ptr(adminToken), map[string]string{
		"username": "public-id-peer",
		"password": "Peerpass1",
	})
	assertStatus(t, userResp, http.StatusCreated)
	peerEntity := parseOK(t, userResp)
	peerPublicID := peerEntity["public_id"].(string)
	peerToken := login(t, "public-id-peer", "Peerpass1")

	reqResp := doJSON(t, "POST", "/api/v1/friends/requests", ptr(adminToken), map[string]any{
		"source_public_id": botPublicID,
		"target_public_id": peerPublicID,
	})
	assertStatus(t, reqResp, http.StatusCreated)
	reqID := int(parseOK(t, reqResp)["id"].(float64))

	acceptResp := doJSON(t, "POST", fmt.Sprintf("/api/v1/friends/requests/%d/accept", reqID), ptr(peerToken), nil)
	assertStatus(t, acceptResp, http.StatusOK)

	convResp := doJSON(t, "POST", "/api/v1/conversations", ptr(adminToken), map[string]any{
		"source_public_id":       botPublicID,
		"title":                  "Bot -> Peer by public_id",
		"conv_type":              "direct",
		"participant_public_ids": []string{peerPublicID},
	})
	assertStatus(t, convResp, http.StatusCreated)
	conv := parseOK(t, convResp)
	participants := conv["participants"].([]interface{})
	if len(participants) != 2 {
		t.Fatalf("expected 2 participants, got %d", len(participants))
	}
	for _, raw := range participants {
		participant := raw.(map[string]interface{})
		if participant["entity_public_id"] == "" {
			t.Fatalf("expected participant entity_public_id, got %v", participant)
		}
	}
}

func TestCreateGroupConversation(t *testing.T) {
	truncateAll(t)
	token := seedAdmin(t)

	// Create a bot to add as participant
	resp := doJSON(t, "POST", "/api/v1/entities", ptr(token), map[string]string{"name": "group-bot"})
	assertStatus(t, resp, http.StatusCreated)
	botData := parseOK(t, resp)
	botEntity, _ := botData["entity"].(map[string]interface{})
	botID := botEntity["id"].(float64)

	// Create group with the bot
	resp = doJSON(t, "POST", "/api/v1/conversations", ptr(token), map[string]interface{}{
		"title":           "Group Chat",
		"conv_type":       "group",
		"participant_ids": []float64{botID},
	})
	assertStatus(t, resp, http.StatusCreated)
}

func TestListConversations(t *testing.T) {
	truncateAll(t)
	token := seedAdmin(t)

	resp := doJSON(t, "POST", "/api/v1/conversations", ptr(token), map[string]interface{}{"title": "Conv 1", "conv_type": "group"})
	assertStatus(t, resp, http.StatusCreated)
	conv1 := parseOK(t, resp)
	conv1ID := int(conv1["id"].(float64))

	doJSON(t, "POST", "/api/v1/messages/send", ptr(token), map[string]interface{}{
		"conversation_id": conv1ID,
		"content_type":    "text",
		"layers":          map[string]string{"summary": "Latest hello"},
	})

	doJSON(t, "POST", "/api/v1/conversations", ptr(token), map[string]interface{}{"title": "Conv 2", "conv_type": "group"})

	resp = doJSON(t, "GET", "/api/v1/conversations", ptr(token), nil)
	assertStatus(t, resp, http.StatusOK)

	result := parseResponse(t, resp)
	convs, ok := result["data"].([]interface{})
	if !ok || len(convs) < 2 {
		t.Fatalf("expected at least 2 conversations, got %v", result["data"])
	}

	var found bool
	for _, raw := range convs {
		conv, _ := raw.(map[string]interface{})
		if int(conv["id"].(float64)) != conv1ID {
			continue
		}
		lastMsg, _ := conv["last_message"].(map[string]interface{})
		if lastMsg == nil {
			t.Fatalf("expected last_message for conversation %d", conv1ID)
		}
		layers, _ := lastMsg["layers"].(map[string]interface{})
		if layers == nil || layers["summary"] != "Latest hello" {
			t.Fatalf("expected last_message summary %q, got %v", "Latest hello", lastMsg["layers"])
		}
		found = true
	}
	if !found {
		t.Fatalf("expected conversation %d in response", conv1ID)
	}
}

func TestGetConversation(t *testing.T) {
	truncateAll(t)
	token := seedAdmin(t)

	resp := doJSON(t, "POST", "/api/v1/conversations", ptr(token), map[string]interface{}{"title": "Get Me", "conv_type": "group"})
	assertStatus(t, resp, http.StatusCreated)
	data := parseOK(t, resp)
	convID := int(data["id"].(float64))

	resp = doJSON(t, "GET", fmt.Sprintf("/api/v1/conversations/%d", convID), ptr(token), nil)
	assertStatus(t, resp, http.StatusOK)
}

func TestGetConversationByPublicID(t *testing.T) {
	truncateAll(t)
	token := seedAdmin(t)

	resp := doJSON(t, "POST", "/api/v1/conversations", ptr(token), map[string]interface{}{"title": "Public ID lookup", "conv_type": "group"})
	assertStatus(t, resp, http.StatusCreated)
	data := parseOK(t, resp)
	meta, _ := data["metadata"].(map[string]interface{})
	publicID, _ := meta["public_id"].(string)
	if publicID == "" {
		t.Fatal("expected metadata.public_id")
	}

	resp = doJSON(t, "GET", fmt.Sprintf("/api/v1/conversations/by-public-id/%s", publicID), ptr(token), nil)
	assertStatus(t, resp, http.StatusOK)
	got := parseOK(t, resp)
	if got["id"] != data["id"] {
		t.Fatalf("expected conversation id %v, got %v", data["id"], got["id"])
	}
}

func TestGetConversationByPublicIDLegacyRoute(t *testing.T) {
	truncateAll(t)
	token := seedAdmin(t)

	resp := doJSON(t, "POST", "/api/v1/conversations", ptr(token), map[string]interface{}{"title": "Legacy Public ID route", "conv_type": "group"})
	assertStatus(t, resp, http.StatusCreated)
	data := parseOK(t, resp)
	meta, _ := data["metadata"].(map[string]interface{})
	publicID, _ := meta["public_id"].(string)
	if publicID == "" {
		t.Fatal("expected metadata.public_id")
	}

	resp = doJSON(t, "GET", fmt.Sprintf("/api/v1/conversations/public/%s", publicID), ptr(token), nil)
	assertStatus(t, resp, http.StatusOK)
}

func TestGetConversationByPublicIDForbiddenForNonParticipant(t *testing.T) {
	truncateAll(t)
	adminToken := seedAdmin(t)

	resp := doJSON(t, "POST", "/api/v1/conversations", ptr(adminToken), map[string]interface{}{"title": "Secret", "conv_type": "group"})
	assertStatus(t, resp, http.StatusCreated)
	data := parseOK(t, resp)
	meta, _ := data["metadata"].(map[string]interface{})
	publicID, _ := meta["public_id"].(string)

	resp = doJSON(t, "POST", "/api/v1/admin/users", ptr(adminToken), map[string]string{
		"username": "publicid-other-user",
		"password": "Otherpass1",
	})
	assertStatus(t, resp, http.StatusCreated)
	otherToken := login(t, "publicid-other-user", "Otherpass1")

	resp = doJSON(t, "GET", fmt.Sprintf("/api/v1/conversations/by-public-id/%s", publicID), ptr(otherToken), nil)
	assertStatus(t, resp, http.StatusForbidden)
}

func TestAddRemoveParticipant(t *testing.T) {
	truncateAll(t)
	token := seedAdmin(t)

	// Create bot
	resp := doJSON(t, "POST", "/api/v1/entities", ptr(token), map[string]string{"name": "part-bot"})
	assertStatus(t, resp, http.StatusCreated)
	botData := parseOK(t, resp)
	botEntity, _ := botData["entity"].(map[string]interface{})
	botID := botEntity["id"].(float64)

	// Create conversation
	resp = doJSON(t, "POST", "/api/v1/conversations", ptr(token), map[string]interface{}{"title": "Part Test", "conv_type": "group"})
	assertStatus(t, resp, http.StatusCreated)
	convData := parseOK(t, resp)
	convID := int(convData["id"].(float64))

	// Add participant
	resp = doJSON(t, "POST", fmt.Sprintf("/api/v1/conversations/%d/participants", convID), ptr(token), map[string]interface{}{
		"entity_id": botID,
	})
	assertStatus(t, resp, http.StatusCreated)

	// Remove participant
	resp = doJSON(t, "DELETE", fmt.Sprintf("/api/v1/conversations/%d/participants/%d", convID, int(botID)), ptr(token), nil)
	assertStatus(t, resp, http.StatusOK)
}

// TestRemoveParticipantRoleCheck verifies that only owner/admin can remove others.
func TestRemoveParticipantRoleCheck(t *testing.T) {
	truncateAll(t)
	token := seedAdmin(t)

	// Create a second user
	resp := doJSON(t, "POST", "/api/v1/admin/users", ptr(token), map[string]string{
		"username": "member-user",
		"password": "Memberpass1",
	})
	assertStatus(t, resp, http.StatusCreated)
	memberData := parseOK(t, resp)
	memberID := int(memberData["id"].(float64))

	memberToken := login(t, "member-user", "Memberpass1")

	// Create a bot
	resp = doJSON(t, "POST", "/api/v1/entities", ptr(token), map[string]string{"name": "target-bot"})
	assertStatus(t, resp, http.StatusCreated)
	botData := parseOK(t, resp)
	botEntity, _ := botData["entity"].(map[string]interface{})
	botID := int(botEntity["id"].(float64))

	// Create conversation (admin is owner)
	resp = doJSON(t, "POST", "/api/v1/conversations", ptr(token), map[string]interface{}{
		"title":           "Role Test",
		"conv_type":       "group",
		"participant_ids": []float64{float64(memberID), float64(botID)},
	})
	assertStatus(t, resp, http.StatusCreated)
	convData := parseOK(t, resp)
	convID := int(convData["id"].(float64))

	// Member (not owner/admin) tries to remove bot — should fail
	resp = doJSON(t, "DELETE", fmt.Sprintf("/api/v1/conversations/%d/participants/%d", convID, botID), ptr(memberToken), nil)
	assertStatus(t, resp, http.StatusForbidden)

	// Owner removes bot — should succeed
	resp = doJSON(t, "DELETE", fmt.Sprintf("/api/v1/conversations/%d/participants/%d", convID, botID), ptr(token), nil)
	assertStatus(t, resp, http.StatusOK)

	// Member removes self — should succeed
	resp = doJSON(t, "DELETE", fmt.Sprintf("/api/v1/conversations/%d/participants/%d", convID, memberID), ptr(memberToken), nil)
	assertStatus(t, resp, http.StatusOK)
}

// TestAddParticipantAdminRoleElevation verifies that only owner/admin can assign admin role.
func TestAddParticipantAdminRoleElevation(t *testing.T) {
	truncateAll(t)
	token := seedAdmin(t)

	// Create a regular member
	resp := doJSON(t, "POST", "/api/v1/admin/users", ptr(token), map[string]string{
		"username": "member",
		"password": "Memberpass1",
	})
	assertStatus(t, resp, http.StatusCreated)
	memberData := parseOK(t, resp)
	memberID := int(memberData["id"].(float64))
	memberToken := login(t, "member", "Memberpass1")

	// Create a bot to add
	resp = doJSON(t, "POST", "/api/v1/entities", ptr(token), map[string]string{"name": "role-test-bot"})
	assertStatus(t, resp, http.StatusCreated)
	botData := parseOK(t, resp)
	botEntity, _ := botData["entity"].(map[string]interface{})
	botID := int(botEntity["id"].(float64))

	// Create conversation (admin is owner, member is participant)
	resp = doJSON(t, "POST", "/api/v1/conversations", ptr(token), map[string]interface{}{
		"title":           "Role Elevation Test",
		"conv_type":       "group",
		"participant_ids": []float64{float64(memberID)},
	})
	assertStatus(t, resp, http.StatusCreated)
	convData := parseOK(t, resp)
	convID := int(convData["id"].(float64))

	// Member tries to add bot as admin — should be forbidden
	resp = doJSON(t, "POST", fmt.Sprintf("/api/v1/conversations/%d/participants", convID), ptr(memberToken), map[string]interface{}{
		"entity_id": botID,
		"role":      "admin",
	})
	assertStatus(t, resp, http.StatusForbidden)

	// Owner adds bot as admin — should succeed
	resp = doJSON(t, "POST", fmt.Sprintf("/api/v1/conversations/%d/participants", convID), ptr(token), map[string]interface{}{
		"entity_id": botID,
		"role":      "admin",
	})
	assertStatus(t, resp, http.StatusCreated)

	// Member tries to add another entity as regular member — should be forbidden (only owner/admin can add)
	resp = doJSON(t, "POST", "/api/v1/entities", ptr(token), map[string]string{"name": "regular-bot"})
	assertStatus(t, resp, http.StatusCreated)
	botData2 := parseOK(t, resp)
	botEntity2, _ := botData2["entity"].(map[string]interface{})
	botID2 := int(botEntity2["id"].(float64))

	resp = doJSON(t, "POST", fmt.Sprintf("/api/v1/conversations/%d/participants", convID), ptr(memberToken), map[string]interface{}{
		"entity_id": botID2,
	})
	assertStatus(t, resp, http.StatusForbidden)
}

func TestMemberCanAddOwnedBotToConversation(t *testing.T) {
	truncateAll(t)
	adminToken := seedAdmin(t)

	resp := doJSON(t, "POST", "/api/v1/admin/users", ptr(adminToken), map[string]string{
		"username": "member-owner",
		"password": "Memberpass1",
	})
	assertStatus(t, resp, http.StatusCreated)
	memberData := parseOK(t, resp)
	memberID := int(memberData["id"].(float64))
	memberToken := login(t, "member-owner", "Memberpass1")

	resp = doJSON(t, "POST", "/api/v1/entities", ptr(memberToken), map[string]string{"name": "member-owned-bot"})
	assertStatus(t, resp, http.StatusCreated)
	botData := parseOK(t, resp)
	botEntity, _ := botData["entity"].(map[string]interface{})
	botID := int(botEntity["id"].(float64))

	resp = doJSON(t, "POST", "/api/v1/conversations", ptr(adminToken), map[string]interface{}{
		"title":           "Owned Bot Join",
		"conv_type":       "group",
		"participant_ids": []float64{float64(memberID)},
	})
	assertStatus(t, resp, http.StatusCreated)
	convData := parseOK(t, resp)
	convID := int(convData["id"].(float64))

	resp = doJSON(t, "POST", fmt.Sprintf("/api/v1/conversations/%d/participants", convID), ptr(memberToken), map[string]interface{}{
		"entity_id": botID,
	})
	assertStatus(t, resp, http.StatusCreated)
}

func TestMemberCannotAddOtherUsersBotToConversation(t *testing.T) {
	truncateAll(t)
	adminToken := seedAdmin(t)

	resp := doJSON(t, "POST", "/api/v1/admin/users", ptr(adminToken), map[string]string{
		"username": "member-other-bot",
		"password": "Memberpass1",
	})
	assertStatus(t, resp, http.StatusCreated)
	memberData := parseOK(t, resp)
	memberID := int(memberData["id"].(float64))
	memberToken := login(t, "member-other-bot", "Memberpass1")

	resp = doJSON(t, "POST", "/api/v1/entities", ptr(adminToken), map[string]string{"name": "admin-owned-bot"})
	assertStatus(t, resp, http.StatusCreated)
	botData := parseOK(t, resp)
	botEntity, _ := botData["entity"].(map[string]interface{})
	botID := int(botEntity["id"].(float64))

	resp = doJSON(t, "POST", "/api/v1/conversations", ptr(adminToken), map[string]interface{}{
		"title":           "Foreign Bot Join",
		"conv_type":       "group",
		"participant_ids": []float64{float64(memberID)},
	})
	assertStatus(t, resp, http.StatusCreated)
	convData := parseOK(t, resp)
	convID := int(convData["id"].(float64))

	resp = doJSON(t, "POST", fmt.Sprintf("/api/v1/conversations/%d/participants", convID), ptr(memberToken), map[string]interface{}{
		"entity_id": botID,
	})
	assertStatus(t, resp, http.StatusForbidden)
}

func TestMemberCannotAddOwnedBotToDirectConversation(t *testing.T) {
	truncateAll(t)
	adminToken := seedAdmin(t)

	resp := doJSON(t, "POST", "/api/v1/admin/users", ptr(adminToken), map[string]string{
		"username": "member-direct-bot",
		"password": "Memberpass1",
	})
	assertStatus(t, resp, http.StatusCreated)
	memberData := parseOK(t, resp)
	memberID := int(memberData["id"].(float64))
	memberToken := login(t, "member-direct-bot", "Memberpass1")

	resp = doJSON(t, "POST", "/api/v1/entities", ptr(memberToken), map[string]string{"name": "member-direct-owned-bot"})
	assertStatus(t, resp, http.StatusCreated)
	botData := parseOK(t, resp)
	botEntity, _ := botData["entity"].(map[string]interface{})
	botID := int(botEntity["id"].(float64))

	resp = doJSON(t, "POST", "/api/v1/friends/requests", ptr(adminToken), map[string]interface{}{
		"target_entity_id": memberID,
	})
	assertStatus(t, resp, http.StatusCreated)
	friendReqID := int(parseOK(t, resp)["id"].(float64))

	resp = doJSON(t, "POST", fmt.Sprintf("/api/v1/friends/requests/%d/accept", friendReqID), ptr(memberToken), nil)
	assertStatus(t, resp, http.StatusOK)

	resp = doJSON(t, "POST", "/api/v1/conversations", ptr(adminToken), map[string]interface{}{
		"title":           "Owned Bot Direct",
		"conv_type":       "direct",
		"participant_ids": []float64{float64(memberID)},
	})
	assertStatus(t, resp, http.StatusCreated)
	convData := parseOK(t, resp)
	convID := int(convData["id"].(float64))

	resp = doJSON(t, "POST", fmt.Sprintf("/api/v1/conversations/%d/participants", convID), ptr(memberToken), map[string]interface{}{
		"entity_id": botID,
	})
	assertStatus(t, resp, http.StatusForbidden)
}

func TestUpdateSubscription(t *testing.T) {
	truncateAll(t)
	token := seedAdmin(t)

	// Create conversation
	resp := doJSON(t, "POST", "/api/v1/conversations", ptr(token), map[string]interface{}{"title": "Sub Test"})
	assertStatus(t, resp, http.StatusCreated)
	convData := parseOK(t, resp)
	convID := int(convData["id"].(float64))

	// Update subscription to subscribe_all
	resp = doJSON(t, "PUT", fmt.Sprintf("/api/v1/conversations/%d/subscription", convID), ptr(token), map[string]string{
		"mode": "subscribe_all",
	})
	assertStatus(t, resp, http.StatusOK)
	data := parseOK(t, resp)
	if data["mode"] != "subscribe_all" {
		t.Fatalf("expected mode=subscribe_all, got %v", data["mode"])
	}

	// Update back to mention_only
	resp = doJSON(t, "PUT", fmt.Sprintf("/api/v1/conversations/%d/subscription", convID), ptr(token), map[string]string{
		"mode": "mention_only",
	})
	assertStatus(t, resp, http.StatusOK)

	// Invalid mode should fail
	resp = doJSON(t, "PUT", fmt.Sprintf("/api/v1/conversations/%d/subscription", convID), ptr(token), map[string]string{
		"mode": "invalid_mode",
	})
	assertStatus(t, resp, http.StatusBadRequest)
}
