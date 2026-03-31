// Package storage provides persistent storage backends for IMAP server.
//
// FileStorage saves data to disk with atomic operations:
//   - Users: data/users/*.json
//   - Mailboxes: data/mailboxes/{username}/*.json
//   - Messages: data/messages/{username}/{mailbox}/msg_*.eml
//
// Atomic writes: write to .tmp file, then os.Rename.
package storage
