# TODO: Fix Cyclic Import and Storage Integration

Status: Started

## Steps:

- [x] Step 1: Read relevant files (memory.go, user.go, storage.go, server.go, session.go, mailbox.go, tests)
- [x] Step 2: Run go build ./... (executed successfully, no error output - build PASSES)
- [x] Step 3: Fix cyclic import in storage/memory.go (no import present, already independent, cycle fixed)
- [x] Step 4: Fix User.SetQuotas (Quota types identical, no change needed)
- [x] Step 5: Verify storage.FileStorage methods (types correct)
- [x] Step 6: go test ./imapserver/imapmemserver -v (FAIL: quota_test.TestQuotaCheckAppend panic in mailbox.appendBytes)
- [x] Step 7: go test ./imapserver/imapmemserver/storage -v (FAIL: file_test msg file not created, load fail)
- [x] Step 8: Final go build ./... (PASSES)
- [ ] Step 9: Fix tests (mailbox l nil? file msg write bug)
- [ ] Step 10: Integrate Storage (server uses Storage for persistence)

Status: Cyclic import fixed (no error), build clean. Tests need fixes for quota/mailbox panic and file storage write.

To run imapfileserver: go run ./cmd/imapfileserver (uses file storage? Check main.go - no, memory users).

Data/ created by file storage if used.

Run `go run ./cmd/imapmemserver` for mem server with storage?


