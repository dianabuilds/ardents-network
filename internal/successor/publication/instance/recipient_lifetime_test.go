package instance

import (
	"testing"
	"testing/synctest"
)

// This is an erasure/join control only. It supplies no accepted Credential,
// registration, key use, publication or recipient-opening authority.
func TestRecipientCloseRetainsKeyUntilOriginalUserReturns(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		root := &Root{}
		binding := &Binding{root: root}
		recipient := &Recipient{binding: binding, private: make([]byte, 32)}
		binding.recipients[0] = recipient
		recipient.users.Add(1)
		returned := false
		defer func() {
			if !returned {
				recipient.users.Done()
			}
		}()
		joined := make(chan error, 1)
		go func() { joined <- recipient.Close() }()
		synctest.Wait()
		root.mu.Lock()
		retained := recipient.closed && len(recipient.private) == 32 && binding.recipients[0] == recipient
		root.mu.Unlock()
		if !retained {
			t.Error("retirement erased key or returned recipient capacity before original user joined")
		}
		early := false
		select {
		case err := <-joined:
			early = true
			t.Error("recipient Close returned before original user", err)
		default:
		}
		recipient.users.Done()
		returned = true
		synctest.Wait()
		if !early {
			select {
			case err := <-joined:
				if err != nil {
					t.Fatal(err)
				}
			default:
				t.Fatal("recipient did not join after original user returned")
			}
		}
		if len(recipient.private) != 0 || binding.recipients[0] != nil || recipient.Close() != nil {
			t.Fatal("joined recipient retained private key/capacity or changed Close outcome")
		}
	})
}
