package ws

import (
	"testing"
	"time"
)

func TestPresenceState(t *testing.T) {
	hub := NewHub(nil)

	if got := hub.PresenceState(42); got != "offline" {
		t.Fatalf("expected offline for unseen entity, got %q", got)
	}

	hub.lastSeen[42] = time.Now().Add(-5 * time.Minute)
	if got := hub.PresenceState(42); got != "recently_online" {
		t.Fatalf("expected recently_online, got %q", got)
	}

	hub.lastSeen[42] = time.Now().Add(-20 * time.Minute)
	if got := hub.PresenceState(42); got != "offline" {
		t.Fatalf("expected offline after stale last_seen, got %q", got)
	}

	client := &Client{entityID: 42}
	hub.clients[client] = true
	hub.lastConnected[42] = time.Now()
	if got := hub.PresenceState(42); got != "online" {
		t.Fatalf("expected online with active client, got %q", got)
	}
	if _, ok := hub.LastConnected(42); !ok {
		t.Fatal("expected last connected timestamp")
	}
}
