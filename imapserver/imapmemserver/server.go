package imapmemserver

import (
    "sync"

    "github.com/emersion/go-imap/v2/imapserver"
    "github.com/emersion/go-imap/v2/imapserver/imapmemserver/storage"
)

// Server is a server instance.
type Server struct {
    mutex   sync.Mutex
    users   map[string]*User
    storage storage.Storage
}

// New creates a new server with in-memory storage.
func New() *Server {
    return NewWithStorage(nil)
}

// NewWithStorage creates a new server with a custom storage backend.
func NewWithStorage(st storage.Storage) *Server {
    return &Server{
        users:   make(map[string]*User),
        storage: st,
    }
}

// NewSession creates a new IMAP session.
func (s *Server) NewSession() imapserver.Session {
    return &serverSession{server: s}
}

func (s *Server) user(username string) *User {
    s.mutex.Lock()
    defer s.mutex.Unlock()
    return s.users[username]
}

// AddUser adds a user to the server and saves to storage.
func (s *Server) AddUser(user *User) {
    s.mutex.Lock()
    defer s.mutex.Unlock()

    // Save to storage if available
    if s.storage != nil {
        quotas := storage.Quota{
            MaxMessages:        user.quotas.MaxMessages,
            MaxStorageBytes:    user.quotas.MaxStorageBytes,
            MaxMessagesPerHour: user.quotas.MaxMessagesPerHour,
        }
        // Create user in storage
        if err := s.storage.CreateUser(user.username, user.password, quotas); err != nil {
            // Log error but continue
            println("Failed to save user to storage:", err.Error())
        }
    }

    s.users[user.username] = user
}

type serverSession struct {
    *UserSession // may be nil
    server       *Server // immutable
}

var _ imapserver.Session = (*serverSession)(nil)

func (sess *serverSession) Login(username, password string) error {
    u := sess.server.user(username)
    if u == nil {
        return imapserver.ErrAuthFailed
    }
    if err := u.Login(username, password); err != nil {
        return err
    }
    sess.UserSession = NewUserSession(u)
    return nil
}