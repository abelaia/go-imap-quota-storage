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
	s := &Server{
		users:   make(map[string]*User),
		storage: st,
	}

	// Auto-load users from storage
	if st != nil {
		usernames, err := st.ListUsers()
		if err != nil {
			println("Failed to list users from storage:", err.Error())
			return s
		}

		for _, username := range usernames {
			ud, err := st.GetUser(username)
			if err != nil {
				println("Failed to load user", username, ":", err.Error())
				continue
			}

			u := NewUserUnlimited(ud.Username, ud.PasswordHash)
			u.SetQuotas(Quota{
				MaxMessages:        ud.Quotas.MaxMessages,
				MaxStorageBytes:    ud.Quotas.MaxStorageBytes,
				MaxMessagesPerHour: ud.Quotas.MaxMessagesPerHour,
			})
			u.server = s

			mailboxNames, err := st.ListMailboxes(username)
			if err != nil {
				println("Failed to list mailboxes for", username, ":", err.Error())
			} else {
				for _, mboxName := range mailboxNames {
					err := u.Create(mboxName, nil)
					if err != nil {
						println("Failed to recreate mailbox", mboxName, ":", err.Error())
					}
				}
			}

			s.AddUser(u)
		}
	}

	return s
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

	user.server = s
	s.users[user.username] = user
}

type serverSession struct {
	*UserSession         // may be nil
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
	u.server = sess.server
	sess.UserSession = NewUserSession(u)
	return nil
}
