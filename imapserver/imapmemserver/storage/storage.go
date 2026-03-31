package storage

import "time"

type Storage interface {
	// User management
	GetUser(username string) (*UserData, error)
	CreateUser(username, passwordHash string, quotas Quota) error
	UpdateUserQuota(username string, quotas Quota) error

	// Mailbox management
	GetMailbox(username, mailboxName string) (*MailboxData, error)
	CreateMailbox(username, mailboxName string, uidValidity uint32) error
	DeleteMailbox(username, mailboxName string) error
	RenameMailbox(username, oldName, newName string) error
	ListMailboxes(username string) ([]string, error)

	// Message management
	AppendMessage(username, mailboxName string, uid uint32, data []byte, flags []string, t time.Time) error
	GetMessages(username, mailboxName string) ([]MessageData, error)
	DeleteMessage(username, mailboxName string, uid uint32) error
}

type Quota struct {
	MaxMessages        uint32 `json:"max_messages"`
	MaxStorageBytes    uint64 `json:"max_storage_bytes"`
	MaxMessagesPerHour uint32 `json:"max_messages_per_hour"`
}

type UserData struct {
	Username     string          `json:"username"`
	PasswordHash string          `json:"password_hash"`
	Quotas       Quota           `json:"quotas"`
	Mailboxes    map[string]bool `json:"mailboxes"`
}

type MailboxData struct {
	Name        string        `json:"name"`
	UIDValidity uint32        `json:"uid_validity"`
	UIDNext     uint32        `json:"uid_next"`
	Messages    []MessageData `json:"messages"`
}

type MessageData struct {
	UID   uint32    `json:"uid"`
	Flags []string  `json:"flags"`
	Time  time.Time `json:"time"`
	Body  []byte    `json:"body"`
}
