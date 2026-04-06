package storage

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"
)

type FileStorage struct {
	mu      sync.RWMutex
	baseDir string
}

func NewFileStorage(dir string) *FileStorage {
	if dir == "" {
		dir = "data"
	}
	s := &FileStorage{baseDir: dir}
	s.ensureDirs()
	return s
}

func (s *FileStorage) ensureDirs() {
	os.MkdirAll(filepath.Join(s.baseDir, "users"), 0755)
	os.MkdirAll(filepath.Join(s.baseDir, "mailboxes"), 0755)
	os.MkdirAll(filepath.Join(s.baseDir, "messages"), 0755)
}

func (s *FileStorage) userPath(username string) string {
	return filepath.Join(s.baseDir, "users", username+".json")
}

func (s *FileStorage) mailboxPath(username, mailboxName string) string {
	return filepath.Join(s.baseDir, "mailboxes", username, mailboxName+".json")
}

func (s *FileStorage) messagePath(username, mailboxName string, uid uint32) string {
	return filepath.Join(s.baseDir, "messages", username, mailboxName, fmt.Sprintf("msg_%d.eml", uid))
}

func (s *FileStorage) atomicWrite(path string, data []byte) error {
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, data, 0644); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}

func (s *FileStorage) loadJSON(path string, v interface{}) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	return json.Unmarshal(data, v)
}

func (s *FileStorage) saveJSON(path string, v interface{}) error {
	data, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return err
	}
	return s.atomicWrite(path, data)
}

func (s *FileStorage) GetUser(username string) (*UserData, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	var ud UserData
	if err := s.loadJSON(s.userPath(username), &ud); err != nil {
		return nil, err
	}
	return &ud, nil
}

func (s *FileStorage) CreateUser(username, passwordHash string, quotas Quota) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	path := s.userPath(username)
	if _, err := os.Stat(path); err == nil {
		return nil // exists
	}
	ud := UserData{
		Username:     username,
		PasswordHash: passwordHash,
		Quotas:       quotas,
		Mailboxes:    map[string]bool{},
	}
	return s.saveJSON(path, ud)
}

func (s *FileStorage) UpdateUserQuota(username string, quotas Quota) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	var ud UserData
	if err := s.loadJSON(s.userPath(username), &ud); err != nil {
		return err
	}
	ud.Quotas = quotas
	return s.saveJSON(s.userPath(username), ud)
}

func (s *FileStorage) GetMailbox(username, mailboxName string) (*MailboxData, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	path := s.mailboxPath(username, mailboxName)
	var md MailboxData
	if err := s.loadJSON(path, &md); err != nil {
		return nil, err
	}
	for i := range md.Messages {
		msgPath := s.messagePath(username, mailboxName, md.Messages[i].UID)
		body, err := os.ReadFile(msgPath)
		if err == nil {
			md.Messages[i].Body = body
		}
	}
	return &md, nil
}

func (s *FileStorage) CreateMailbox(username, mailboxName string, uidValidity uint32) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	userPath := s.userPath(username)
	var ud UserData
	if err := s.loadJSON(userPath, &ud); err != nil {
		return err
	}
	ud.Mailboxes[mailboxName] = true
	if err := s.saveJSON(userPath, ud); err != nil {
		return err
	}
	mboxPath := s.mailboxPath(username, mailboxName)
	if err := os.MkdirAll(filepath.Dir(mboxPath), 0755); err != nil {
		return err
	}
	os.MkdirAll(filepath.Dir(s.messagePath(username, mailboxName, 1)), 0755)
	md := MailboxData{
		Name:        mailboxName,
		UIDValidity: uidValidity,
		UIDNext:     1,
		Messages:    []MessageData{},
	}
	return s.saveJSON(mboxPath, md)
}

func (s *FileStorage) DeleteMailbox(username, mailboxName string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	userPath := s.userPath(username)
	var ud UserData
	if err := s.loadJSON(userPath, &ud); err != nil {
		return err
	}
	delete(ud.Mailboxes, mailboxName)
	s.saveJSON(userPath, ud)
	os.Remove(s.mailboxPath(username, mailboxName))
	os.RemoveAll(filepath.Dir(s.messagePath(username, mailboxName, 1)))
	return nil
}

func (s *FileStorage) RenameMailbox(username, oldName, newName string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	var ud UserData
	if err := s.loadJSON(s.userPath(username), &ud); err != nil {
		return err
	}
	ud.Mailboxes[newName] = true
	delete(ud.Mailboxes, oldName)
	s.saveJSON(s.userPath(username), ud)
	os.Rename(s.mailboxPath(username, oldName), s.mailboxPath(username, newName))
	os.Rename(filepath.Dir(s.messagePath(username, oldName, 1)), filepath.Dir(s.messagePath(username, newName, 1)))
	return nil
}

func (s *FileStorage) ListUsers() ([]string, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	dir := filepath.Join(s.baseDir, "users")
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}
	var users []string
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		name := strings.TrimSuffix(entry.Name(), ".json")
		if name != "" {
			users = append(users, name)
		}
	}
	sort.Strings(users)
	return users, nil
}

func (s *FileStorage) ListMailboxes(username string) ([]string, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	var ud UserData
	if err := s.loadJSON(s.userPath(username), &ud); err != nil {
		return nil, err
	}
	names := make([]string, 0, len(ud.Mailboxes))
	for n := range ud.Mailboxes {
		names = append(names, n)
	}
	sort.Strings(names)
	return names, nil
}

func (s *FileStorage) AppendMessage(username, mailboxName string, uid uint32, data []byte, flags []string, t time.Time) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	var md MailboxData
	if err := s.loadJSON(s.mailboxPath(username, mailboxName), &md); err != nil {
		return err
	}
	var ud UserData
	if err := s.loadJSON(s.userPath(username), &ud); err != nil {
		return err
	}
	storageUsed := uint64(0)
	for _, m := range md.Messages {
		storageUsed += uint64(len(m.Body))
	}
	storageUsed += uint64(len(data))
	if ud.Quotas.MaxMessages > 0 && uint32(len(md.Messages)) >= ud.Quotas.MaxMessages {
		return fmt.Errorf("quota exceeded: max_messages")
	}
	if ud.Quotas.MaxStorageBytes > 0 && storageUsed > ud.Quotas.MaxStorageBytes {
		return fmt.Errorf("quota exceeded: max_storage_bytes")
	}
	if uid == 0 {
		uid = md.UIDNext
		md.UIDNext++
	}
	msgData := MessageData{
		UID:   uid,
		Flags: flags,
		Time:  t,
		Body:  data,
	}
	md.Messages = append(md.Messages, msgData)
	msgPath := s.messagePath(username, mailboxName, uid)
	if err := os.MkdirAll(filepath.Dir(msgPath), 0755); err != nil {
		return err
	}
	if err := s.atomicWrite(msgPath, data); err != nil {
		return err
	}
	return s.saveJSON(s.mailboxPath(username, mailboxName), md)
}

func (s *FileStorage) GetMessages(username, mailboxName string) ([]MessageData, error) {
	md, err := s.GetMailbox(username, mailboxName)
	if err != nil {
		return nil, err
	}
	return md.Messages, nil
}

func (s *FileStorage) DeleteMessage(username, mailboxName string, uid uint32) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	var md MailboxData
	if err := s.loadJSON(s.mailboxPath(username, mailboxName), &md); err != nil {
		return err
	}
	for i, m := range md.Messages {
		if m.UID == uid {
			md.Messages = append(md.Messages[:i], md.Messages[i+1:]...)
			os.Remove(s.messagePath(username, mailboxName, uid))
			return s.saveJSON(s.mailboxPath(username, mailboxName), md)
		}
	}
	return nil
}
