package main

import (
	"log"
	"net"

	"github.com/emersion/go-imap/v2"
	"github.com/emersion/go-imap/v2/imapserver"
	"github.com/emersion/go-imap/v2/imapserver/imapmemserver"
	"github.com/emersion/go-imap/v2/imapserver/imapmemserver/storage"
)

func main() {
	// Create file storage (saves to disk)
	fs := storage.NewFileStorage("data")

	// Create server with file storage
	server := imapmemserver.NewWithStorage(fs)

	// Create test user with quotas
	user := imapmemserver.NewUser("alice", "password")
	user.SetQuotas(imapmemserver.Quota{
		MaxMessages:     100,
		MaxStorageBytes: 10 * 1024 * 1024, // 10 MB
	})
	server.AddUser(user)

	// Create INBOX mailbox
	if err := user.Create("INBOX", nil); err != nil {
		log.Printf("Warning: failed to create INBOX: %v", err)
	}

	// Start listening
	ln, err := net.Listen("tcp", ":143")
	if err != nil {
		log.Fatal(err)
	}
	defer ln.Close()

	log.Println("IMAP server with FILE STORAGE on :143")
	log.Println("User: alice, Password: password")
	log.Println("Quotas: max 100 messages, 10 MB storage")
	log.Println("Data directory: ./data/")

	// Create IMAP server
	imapSrv := imapserver.New(&imapserver.Options{
		NewSession: func(conn *imapserver.Conn) (imapserver.Session, *imapserver.GreetingData, error) {
			return server.NewSession(), nil, nil
		},
		Caps: imap.CapSet{
			imap.CapIMAP4rev1: {},
			imap.CapIMAP4rev2: {},
		},
		InsecureAuth: true,
	})

	if err := imapSrv.Serve(ln); err != nil {
		log.Fatal(err)
	}
}
