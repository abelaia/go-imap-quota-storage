package imapmemserver

import (
    "testing"
)

func TestNewServer(t *testing.T) {
    s := New()
    if s == nil {
        t.Fatal("New() returned nil")
    }
}

func TestAddUser(t *testing.T) {
    s := New()
    u := NewUser("testuser", "pass")
    s.AddUser(u)
    
    if s.user("testuser") == nil {
        t.Error("User not found after AddUser")
    }
}

func TestCreateMailbox(t *testing.T) {
    s := New()
    u := NewUser("alice", "pass")
    s.AddUser(u)
    
    err := u.Create("INBOX", nil)
    if err != nil {
        t.Errorf("Create mailbox failed: %v", err)
    }
}
