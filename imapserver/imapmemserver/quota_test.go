package imapmemserver

import (
	"bytes"
	"strings"
	"testing"
)

func TestQuotaDefaults(t *testing.T) {
	u := NewUser("test", "pass")

	q := u.GetQuotas()
	if q.MaxMessages != 0 || q.MaxStorageBytes != 0 || q.MaxMessagesPerHour != 0 {
		t.Errorf("Expected default quotas to be zero, got %+v", q)
	}
}

func TestSetQuotas(t *testing.T) {
	u := NewUser("test", "pass")

	newQuota := Quota{
		MaxMessages:        100,
		MaxStorageBytes:    1024 * 1024,
		MaxMessagesPerHour: 50,
	}
	u.SetQuotas(newQuota)

	q := u.GetQuotas()
	if q.MaxMessages != 100 {
		t.Errorf("Expected MaxMessages=100, got %d", q.MaxMessages)
	}
	if q.MaxStorageBytes != 1024*1024 {
		t.Errorf("Expected MaxStorageBytes=1048576, got %d", q.MaxStorageBytes)
	}
	if q.MaxMessagesPerHour != 50 {
		t.Errorf("Expected MaxMessagesPerHour=50, got %d", q.MaxMessagesPerHour)
	}
}

func TestQuotaCheckAppend(t *testing.T) {
	u := NewUser("test", "pass")

	// Set quota: max 2 messages
	u.SetQuotas(Quota{MaxMessages: 2})

	// Create mailbox
	err := u.Create("INBOX", nil)
	if err != nil {
		t.Fatal(err)
	}

	// Add 2 messages (should succeed)
	for i := 0; i < 2; i++ {
		r := bytes.NewReader([]byte("Message " + string(rune('A'+i))))
		_, err := u.Append("INBOX", r, nil)
		if err != nil {
			t.Errorf("Append %d failed: %v", i+1, err)
		}
	}

	// Try to add 3rd message (should fail)
	r := bytes.NewReader([]byte("Message 3"))
	_, err = u.Append("INBOX", r, nil)
	if err == nil {
		t.Error("Expected quota exceeded error, got nil")
	}
}

func TestQuotaCheckStorageBytes(t *testing.T) {
	u := NewUser("test", "pass")

	// Set quota: max 100 bytes total
	u.SetQuotas(Quota{MaxStorageBytes: 100})

	err := u.Create("INBOX", nil)
	if err != nil {
		t.Fatal(err)
	}

	// Add 60-byte message
	r := bytes.NewReader([]byte(strings.Repeat("x", 60)))
	_, err = u.Append("INBOX", r, nil)
	if err != nil {
		t.Errorf("First append failed: %v", err)
	}

	// Add 50-byte message (total would be 110 > 100)
	r = bytes.NewReader([]byte(strings.Repeat("y", 50)))
	_, err = u.Append("INBOX", r, nil)
	if err == nil {
		t.Error("Expected quota exceeded error (storage), got nil")
	}
}

func TestQuotaCopy(t *testing.T) {
	u := NewUser("test", "pass")
	u.SetQuotas(Quota{MaxMessages: 1})

	// Create source and destination mailboxes
	u.Create("INBOX", nil)
	u.Create("Archive", nil)

	// Add a message to INBOX
	r := bytes.NewReader([]byte("Test message"))
	_, err := u.Append("INBOX", r, nil)
	if err != nil {
		t.Fatal(err)
	}

	// This would test COPY command, but requires session
	// For now, just verify quota is set
	q := u.GetQuotas()
	if q.MaxMessages != 1 {
		t.Errorf("Quota not set correctly: %+v", q)
	}
}
