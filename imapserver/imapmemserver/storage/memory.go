package storage

import (
	"fmt"
	"sort"
	"sync"
	"time"
)

type MemoryStorage struct {
	mu    sync.RWMutex
	users map[string]*memUser
}

type memUser struct {
	quota     Quota
	mailboxes map[string]*memMailbox
}

type memMailbox struct {
	uidValidity uint32
	messages    []MessageData
	uidNext     uint32
}

func NewMemoryStorage() *MemoryStorage {
	return &MemoryStorage{
		users: make(map[string]*memUser),
	}
}

func (s *MemoryStorage) GetUser(username string) (*UserData, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	mu, ok := s.users[username]
	if !ok {
		return nil, nil
	}

	return &UserData{
		Username:     username,
		PasswordHash: "",
		Quotas:       mu.quota,
		Mailboxes:    make(map[string]bool),
	}, nil
}

func (s *MemoryStorage) CreateUser(username, passwordHash string, quotas Quota) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, exists := s.users[username]; exists {
		return nil
	}

	if quotas.MaxMessages == 0 {
		quotas.MaxMessages = 1000
	}
	if quotas.MaxStorageBytes == 0 {
		quotas.MaxStorageBytes = 100 * 1024 * 1024
	}
	if quotas.MaxMessagesPerHour == 0 {
		quotas.MaxMessagesPerHour = 100
	}

	s.users[username] = &memUser{
		quota:     quotas,
		mailboxes: make(map[string]*memMailbox),
	}
	return nil
}

func (s *MemoryStorage) UpdateUserQuota(username string, quotas Quota) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	mu, ok := s.users[username]
	if !ok {
		return nil
	}
	mu.quota = quotas
	return nil
}

func (s *MemoryStorage) GetMailbox(username, mailboxName string) (*MailboxData, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	mu, ok := s.users[username]
	if !ok {
		return nil, nil
	}

	mbox, ok := mu.mailboxes[mailboxName]
	if !ok {
		return nil, nil
	}

	return &MailboxData{
		Name:        mailboxName,
		UIDValidity: mbox.uidValidity,
		UIDNext:     mbox.uidNext,
		Messages:    mbox.messages,
	}, nil
}

func (s *MemoryStorage) CreateMailbox(username, mailboxName string, uidValidity uint32) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	mu, ok := s.users[username]
	if !ok {
		return nil
	}

	if _, ok := mu.mailboxes[mailboxName]; ok {
		return nil
	}

	mu.mailboxes[mailboxName] = &memMailbox{
		uidValidity: uidValidity,
		uidNext:     1,
		messages:    []MessageData{},
	}
	return nil
}

func (s *MemoryStorage) DeleteMailbox(username, mailboxName string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	mu, ok := s.users[username]
	if !ok {
		return nil
	}

	delete(mu.mailboxes, mailboxName)
	return nil
}

func (s *MemoryStorage) RenameMailbox(username, oldName, newName string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	mu, ok := s.users[username]
	if !ok {
		return nil
	}

	mbox, ok := mu.mailboxes[oldName]
	if !ok {
		return nil
	}

	mu.mailboxes[newName] = mbox
	delete(mu.mailboxes, oldName)
	return nil
}

func (s *MemoryStorage) ListMailboxes(username string) ([]string, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	mu, ok := s.users[username]
	if !ok {
		return nil, nil
	}

	names := make([]string, 0, len(mu.mailboxes))
	for name := range mu.mailboxes {
		names = append(names, name)
	}
	sort.Strings(names)
	return names, nil
}

func (s *MemoryStorage) AppendMessage(username, mailboxName string, uid uint32, data []byte, flags []string, t time.Time) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	mu, ok := s.users[username]
	if !ok {
		return nil
	}

	mbox, ok := mu.mailboxes[mailboxName]
	if !ok {
		return nil
	}

	if uint32(len(mbox.messages)) >= mu.quota.MaxMessages {
		return fmt.Errorf("quota exceeded")
	}

	if uid == 0 {
		uid = mbox.uidNext
		mbox.uidNext++
	}

	msg := MessageData{
		UID:   uid,
		Flags: flags,
		Time:  t,
		Body:  data,
	}
	mbox.messages = append(mbox.messages, msg)
	return nil
}

func (s *MemoryStorage) GetMessages(username, mailboxName string) ([]MessageData, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	mu, ok := s.users[username]
	if !ok {
		return nil, nil
	}

	mbox, ok := mu.mailboxes[mailboxName]
	if !ok {
		return nil, nil
	}

	msgs := make([]MessageData, len(mbox.messages))
	copy(msgs, mbox.messages)
	return msgs, nil
}

func (s *MemoryStorage) DeleteMessage(username, mailboxName string, uid uint32) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	mu, ok := s.users[username]
	if !ok {
		return nil
	}

	mbox, ok := mu.mailboxes[mailboxName]
	if !ok {
		return nil
	}

	for i, m := range mbox.messages {
		if m.UID == uid {
			mbox.messages = append(mbox.messages[:i], mbox.messages[i+1:]...)
			return nil
		}
	}
	return nil
}
