package application

import (
	"strings"
	"testing"
	"time"
)

func TestUnsubscribeManagerRoundTripTamperingAndExpiry(t *testing.T) {
	manager, err := NewUnsubscribeManager(strings.Repeat("k", 48), "https://auraedugh.vercel.app")
	if err != nil {
		t.Fatal(err)
	}
	issuedAt := time.Date(2026, time.July, 21, 8, 0, 0, 0, time.UTC)
	manager.now = func() time.Time { return issuedAt }
	addressHash := strings.Repeat("a", 64)
	link, err := manager.Link("school-a", addressHash)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(link, "school-a") || strings.Contains(link, "teacher@example.com") || !strings.HasPrefix(link, "https://auraedugh.vercel.app/unsubscribe#token=") {
		t.Fatalf("unsafe unsubscribe link=%q", link)
	}
	token := strings.TrimPrefix(link, "https://auraedugh.vercel.app/unsubscribe#token=")
	tenantID, gotHash, err := manager.Verify(token)
	if err != nil || tenantID != "school-a" || gotHash != addressHash {
		t.Fatalf("claims tenant=%q hash=%q err=%v", tenantID, gotHash, err)
	}
	if _, _, err := manager.Verify(token + "x"); err == nil {
		t.Fatal("tampered unsubscribe token accepted")
	}
	manager.now = func() time.Time { return issuedAt.Add(unsubscribeValidity + time.Minute) }
	if _, _, err := manager.Verify(token); err == nil {
		t.Fatal("expired unsubscribe token accepted")
	}
}
