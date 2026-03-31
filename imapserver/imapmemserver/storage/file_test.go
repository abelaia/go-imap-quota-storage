package storage

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestFileStorage(t *testing.T) {
	tmpDir := t.TempDir()
	fs := NewFileStorage(tmpDir)

	quota := Quota{MaxMessages: 10}
	fs.CreateUser("testuser", "hash", quota)

	fs.CreateMailbox("testuser", "INBOX", 1)

	msgBody := []byte("Hello world")
	fs.AppendMessage("testuser", "INBOX", 0, msgBody, nil, time.Now())

	userPath := filepath.Join(tmpDir, "users", "testuser.json")
	if _, err := os.Stat(userPath); err != nil {
		t.Fatalf("user file not created: %v", err)
	}

	mboxPath := filepath.Join(tmpDir, "mailboxes", "testuser", "INBOX.json")
	if _, err := os.Stat(mboxPath); err != nil {
		t.Fatalf("mailbox file not created: %v", err)
	}

	msgPath := filepath.Join(tmpDir, "messages", "testuser", "INBOX", "msg_1.eml")
	if _, err := os.Stat(msgPath); err != nil {
		t.Fatalf("message file not created: %v", err)
	}
	_ = fs
}

func TestAtomicWrite(t *testing.T) {
	tmpDir := t.TempDir()
	fs := NewFileStorage(tmpDir)

	// Create .tmp without rename
	tmpPath := filepath.Join(tmpDir, "users", "test.tmp")
	data := []byte(`corrupted data`)
	os.WriteFile(tmpPath, data, 0644)

	// New instance should ignore .tmp
	fs2 := NewFileStorage(tmpDir)
	_, err := fs2.GetUser("test")
	if err == nil {
		t.Fatal("should not find corrupted user")
	}
	_ = fs
}

func TestLoadOnStartup(t *testing.T) {
	tmpDir := t.TempDir()
	fs := NewFileStorage(tmpDir)

	quota := Quota{MaxMessages: 10}
	fs.CreateUser("testuser", "hash", quota)
	fs.CreateMailbox("testuser", "INBOX", 1)
	msgBody := []byte("Hello")
	fs.AppendMessage("testuser", "INBOX", 0, msgBody, nil, time.Now())

	_ = fs

	// New storage
	fs2 := NewFileStorage(tmpDir)

	user, err := fs2.GetUser("testuser")
	if err != nil {
		t.Fatal(err)
	}
	if user.Username != "testuser" {
		t.Errorf("wrong username: %s", user.Username)
	}
	if user.Quotas.MaxMessages != 10 {
		t.Errorf("wrong quota: %d", user.Quotas.MaxMessages)
	}

	messages, err := fs2.GetMessages("testuser", "INBOX")
	if err != nil {
		t.Fatal(err)
	}
	if len(messages) != 1 {
		t.Errorf("wrong message count: %d", len(messages))
	}
	if len(messages[0].Body) != len(msgBody) {
		t.Errorf("wrong body size: %d", len(messages[0].Body))
	}
}

func TestQuotaIntegration(t *testing.T) {
	tmpDir := t.TempDir()
	fs := NewFileStorage(tmpDir)

	quota := Quota{MaxMessages: 2}
	fs.CreateUser("testuser", "hash", quota)
	fs.CreateMailbox("testuser", "INBOX", 1)

	fs.AppendMessage("testuser", "INBOX", 0, []byte("1"), nil, time.Now())
	fs.AppendMessage("testuser", "INBOX", 0, []byte("2"), nil, time.Now())

	err := fs.AppendMessage("testuser", "INBOX", 0, []byte("3"), nil, time.Now())
	if err == nil {
		t.Error("should fail quota")
	}

	messages, _ := fs.GetMessages("testuser", "INBOX")
	if len(messages) != 2 {
		t.Errorf("wrong count: %d", len(messages))
	}
	_ = fs
}

func TestCrashRecovery(t *testing.T) {
	tmpDir := t.TempDir()
	fs := NewFileStorage(tmpDir)

	fs.CreateUser("testuser", "hash", Quota{})
	fs.CreateMailbox("testuser", "INBOX", 1)

	msgDir := filepath.Join(tmpDir, "messages", "testuser", "INBOX")
	tmpMsgPath := filepath.Join(msgDir, "msg_1.tmp")
	data := []byte("partial")
	os.WriteFile(tmpMsgPath, data, 0644)

	fs2 := NewFileStorage(tmpDir)
	messages, err := fs2.GetMessages("testuser", "INBOX")
	if err != nil {
		t.Fatal(err)
	}
	if len(messages) != 0 {
		t.Errorf("should have no partial messages: %d", len(messages))
	}
	_ = fs
}
