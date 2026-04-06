package imapmemserver

import (
	"bytes"
	"crypto/subtle"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/emersion/go-imap/v2"
	"github.com/emersion/go-imap/v2/imapserver"
)

const mailboxDelim rune = '/'

type Quota struct {
	MaxMessages        uint32 `json:"max_messages"`
	MaxStorageBytes    uint64 `json:"max_storage_bytes"`
	MaxMessagesPerHour uint32 `json:"max_messages_per_hour"`
}

type User struct {
	username, password string
	quotas             Quota

	mutex             sync.RWMutex
	mailboxes         map[string]*Mailbox
	messageTimestamps []time.Time
	server            *Server
	prevUidValidity   uint32
}

func NewUser(username, password string) *User {
	return &User{
		username:  username,
		password:  password,
		mailboxes: make(map[string]*Mailbox),
		quotas:    Quota{},
	}
}

func NewUserUnlimited(username, password string) *User {
	return NewUser(username, password)
}

func (u *User) SetQuotas(q Quota) {
	u.mutex.Lock()
	defer u.mutex.Unlock()
	u.quotas = q
}

func (u *User) GetQuotas() Quota {
	u.mutex.RLock()
	defer u.mutex.RUnlock()
	return u.quotas
}

func (u *User) Login(username, password string) error {
	if username != u.username {
		return imapserver.ErrAuthFailed
	}
	if subtle.ConstantTimeCompare([]byte(password), []byte(u.password)) != 1 {
		return imapserver.ErrAuthFailed
	}
	return nil
}

func (u *User) mailboxLocked(name string) (*Mailbox, error) {
	mbox := u.mailboxes[name]
	if mbox == nil {
		return nil, &imap.Error{
			Type: imap.StatusResponseTypeNo,
			Code: imap.ResponseCodeNonExistent,
			Text: "No such mailbox",
		}
	}
	return mbox, nil
}

func (u *User) mailbox(name string) (*Mailbox, error) {
	u.mutex.Lock()
	defer u.mutex.Unlock()
	return u.mailboxLocked(name)
}

func (u *User) Status(name string, options *imap.StatusOptions) (*imap.StatusData, error) {
	mbox, err := u.mailbox(name)
	if err != nil {
		return nil, err
	}
	return mbox.StatusData(options), nil
}

func (u *User) List(w *imapserver.ListWriter, ref string, patterns []string, options *imap.ListOptions) error {
	u.mutex.Lock()
	defer u.mutex.Unlock()

	if len(patterns) == 0 {
		return w.WriteList(&imap.ListData{
			Attrs: []imap.MailboxAttr{imap.MailboxAttrNoSelect},
			Delim: mailboxDelim,
		})
	}

	var l []imap.ListData
	for name, mbox := range u.mailboxes {
		match := false
		for _, pattern := range patterns {
			match = imapserver.MatchList(name, mailboxDelim, ref, pattern)
			if match {
				break
			}
		}
		if !match {
			continue
		}

		data := mbox.list(options)
		if data != nil {
			l = append(l, *data)
		}
	}

	sort.Slice(l, func(i, j int) bool {
		return l[i].Mailbox < l[j].Mailbox
	})

	for _, data := range l {
		if err := w.WriteList(&data); err != nil {
			return err
		}
	}

	return nil
}

func (u *User) Append(mailbox string, r imap.LiteralReader, options *imap.AppendOptions) (*imap.AppendData, error) {
	mbox, err := u.mailbox(mailbox)
	if err != nil {
		return nil, err
	}

	// First, read the message to get its size
	var buf bytes.Buffer
	if _, err := buf.ReadFrom(r); err != nil {
		return nil, err
	}
	data := buf.Bytes()

	// Check per-hour quota
	u.mutex.Lock()
	oneHourAgo := time.Now().Add(-time.Hour)
	recent := 0
	for _, t := range u.messageTimestamps {
		if t.After(oneHourAgo) {
			recent++
		}
	}
	if u.quotas.MaxMessagesPerHour > 0 && uint32(recent) >= u.quotas.MaxMessagesPerHour {
		u.mutex.Unlock()
		return nil, &imap.Error{
			Type: imap.StatusResponseTypeNo,
			Code: "QUOTA_EXCEEDED",
			Text: "Quota exceeded: maximum messages per hour",
		}
	}
	u.mutex.Unlock()

	// Check quotas before appending
	mbox.mutex.Lock()
	currentCount := uint32(len(mbox.l))
	currentSize := mbox.sizeLocked()
	mbox.mutex.Unlock()

	if u.quotas.MaxMessages > 0 && currentCount >= u.quotas.MaxMessages {
		return nil, &imap.Error{
			Type: imap.StatusResponseTypeNo,
			Code: "QUOTA_EXCEEDED",
			Text: "Quota exceeded: maximum number of messages",
		}
	}
	if u.quotas.MaxStorageBytes > 0 && uint64(currentSize)+uint64(len(data)) > u.quotas.MaxStorageBytes {
		return nil, &imap.Error{
			Type: imap.StatusResponseTypeNo,
			Code: "QUOTA_EXCEEDED",
			Text: "Quota exceeded: maximum storage size",
		}
	}

	// Create a new reader from the buffered data
	dataReader := bytes.NewReader(data)
	ad, err := mbox.appendLiteral(dataReader, options)
	if err != nil {
		return nil, err
	}

	// Add timestamp
	u.mutex.Lock()
	u.messageTimestamps = append(u.messageTimestamps, time.Now())
	u.mutex.Unlock()

	// Save to storage
	if u.server != nil && u.server.storage != nil {
		flags := make([]string, len(options.Flags))
		for i, f := range options.Flags {
			flags[i] = string(f)
		}
		err := u.server.storage.AppendMessage(u.username, mailbox, uint32(ad.UID), data, flags, options.Time)
		if err != nil {
			println("Failed to append message to storage: ", err.Error())
		}
	}

	return ad, nil
}

func (u *User) Create(name string, options *imap.CreateOptions) error {
	u.mutex.Lock()
	defer u.mutex.Unlock()

	name = strings.TrimRight(name, string(mailboxDelim))

	if u.mailboxes[name] != nil {
		return &imap.Error{
			Type: imap.StatusResponseTypeNo,
			Code: imap.ResponseCodeAlreadyExists,
			Text: "Mailbox already exists",
		}
	}

	u.prevUidValidity++
	u.mailboxes[name] = NewMailbox(name, u.prevUidValidity)

	if u.server != nil && u.server.storage != nil {
		err := u.server.storage.CreateMailbox(u.username, name, u.prevUidValidity)
		if err != nil {
			println("Failed to create mailbox in storage: ", err.Error())
		}
	}
	return nil
}

func (u *User) Delete(name string) error {
	u.mutex.Lock()
	defer u.mutex.Unlock()

	if _, err := u.mailboxLocked(name); err != nil {
		return err
	}

	delete(u.mailboxes, name)

	if u.server != nil && u.server.storage != nil {
		err := u.server.storage.DeleteMailbox(u.username, name)
		if err != nil {
			println("Failed to delete mailbox in storage: ", err.Error())
		}
	}
	return nil
}

func (u *User) Rename(oldName, newName string, options *imap.RenameOptions) error {
	u.mutex.Lock()
	defer u.mutex.Unlock()

	newName = strings.TrimRight(newName, string(mailboxDelim))

	mbox, err := u.mailboxLocked(oldName)
	if err != nil {
		return err
	}

	if u.mailboxes[newName] != nil {
		return &imap.Error{
			Type: imap.StatusResponseTypeNo,
			Code: imap.ResponseCodeAlreadyExists,
			Text: "Mailbox already exists",
		}
	}

	mbox.rename(newName)
	u.mailboxes[newName] = mbox
	delete(u.mailboxes, oldName)

	if u.server != nil && u.server.storage != nil {
		err := u.server.storage.RenameMailbox(u.username, oldName, newName)
		if err != nil {
			println("Failed to rename mailbox in storage: ", err.Error())
		}
	}
	return nil
}

func (u *User) Subscribe(name string) error {
	mbox, err := u.mailbox(name)
	if err != nil {
		return err
	}
	mbox.SetSubscribed(true)
	return nil
}

func (u *User) Unsubscribe(name string) error {
	mbox, err := u.mailbox(name)
	if err != nil {
		return err
	}
	mbox.SetSubscribed(false)
	return nil
}

func (u *User) Namespace() (*imap.NamespaceData, error) {
	return &imap.NamespaceData{
		Personal: []imap.NamespaceDescriptor{{Delim: mailboxDelim}},
	}, nil
}
