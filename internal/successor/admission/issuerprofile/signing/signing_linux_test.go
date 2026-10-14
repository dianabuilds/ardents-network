//go:build linux

package signing

import (
	"bytes"
	"context"
	"crypto/ed25519"
	"crypto/rsa"
	"errors"
	"math/big"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/dianabuilds/ardents-network/internal/successor/admission/issuerprofile"
)

// These public grammar fixtures isolate private signing, not RSA issuance.
// The compiled command scenarios separately use genuine RSA inventories.
func signingRequest(t *testing.T, b Binding) issuerprofile.Request {
	t.Helper()
	var keys []issuerprofile.Key
	for i := 0; i < 3; i++ {
		n := new(big.Int).Lsh(big.NewInt(1), 2047)
		n.Add(n, big.NewInt(int64(i)*2+1))
		spki, e := issuerprofile.EncodeKey(&rsa.PublicKey{N: n, E: 65537})
		if e != nil {
			t.Fatal(e)
		}
		keys = append(keys, issuerprofile.Key{Window: 3600, Class: uint8(i + 1), SPKI: spki})
	}
	request, e := issuerprofile.Prepare(issuerprofile.Binding{Network: b.Network, Issuer: b.Node, Signer: b.Signer, Start: time.Unix(3600, 0).UTC(), End: time.Unix(7200, 0).UTC()}, keys)
	if e != nil {
		t.Fatal(e)
	}
	return request
}

func TestSigningRefusalsLeaveOriginalOwnerUsable(t *testing.T) {
	root, _, b := importedIdentity(t)
	s, e := Open(t.Context(), root, b)
	if e != nil {
		t.Fatal(e)
	}
	defer s.Close()
	request := signingRequest(t, b)
	for _, bad := range []Binding{{Network: [32]byte{9}, Node: b.Node, Signer: b.Signer}, {Network: b.Network, Node: [32]byte{9}, Signer: b.Signer}, {Network: b.Network, Node: b.Node, Signer: [32]byte{9}}} {
		if raw, e := s.SignIssuerProfile(t.Context(), signingRequest(t, bad)); len(raw) != 0 || !errors.Is(e, ErrInvalid) {
			t.Fatal("foreign request", e)
		}
	}
	if raw, e := s.SignIssuerProfile(t.Context(), issuerprofile.Request{}); len(raw) != 0 || !errors.Is(e, ErrInvalid) {
		t.Fatal("zero request", e)
	}
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	if raw, e := s.SignIssuerProfile(ctx, request); len(raw) != 0 || !errors.Is(e, context.Canceled) {
		t.Fatal("original cancellation", e)
	}
	_, transcript, _ := request.Snapshot()
	transcript[0] ^= 1
	raw, e := s.SignIssuerProfile(t.Context(), request)
	if e != nil {
		t.Fatal(e)
	}
	_, actual, _ := request.Snapshot()
	if !ed25519.Verify(b.Signer[:], actual, raw[len(raw)-64:]) {
		t.Fatal("healthy owner or defensive request changed")
	}
}

func TestSigningObservedLossCannotReviveAcrossCopies(t *testing.T) {
	for _, name := range []string{"identity.key", "identity.pin", "identity.lock", "root"} {
		t.Run(name, func(t *testing.T) {
			root, _, b := importedIdentity(t)
			s, e := Open(t.Context(), root, b)
			if e != nil {
				t.Fatal(e)
			}
			copyHandle := s
			request := signingRequest(t, b)
			original := filepath.Join(root, name)
			if name == "root" {
				original = root
			}
			saved := original + ".saved"
			// Keep the old inode outside the root to test restoration without an extra-file refusal.
			if name != "root" {
				saved = filepath.Join(filepath.Dir(root), name+".saved")
			}
			if e = os.Rename(original, saved); e != nil {
				t.Fatal(e)
			}
			if raw, e := s.SignIssuerProfile(t.Context(), request); len(raw) != 0 || !errors.Is(e, ErrUncertain) {
				t.Fatal("loss accepted", e)
			}
			cause := s.state.failure
			if cause == nil {
				t.Fatal("missing retained original cause")
			}
			if e = os.Rename(saved, original); e != nil {
				t.Fatal(e)
			}
			if _, e = copyHandle.Public(); !errors.Is(e, ErrUncertain) {
				t.Fatal("restored Public revived", e)
			}
			if raw, e := copyHandle.SignIssuerProfile(t.Context(), request); len(raw) != 0 || !errors.Is(e, ErrUncertain) {
				t.Fatal("restored signing revived", e)
			}
			first := s.Close()
			if !errors.Is(first, cause) || copyHandle.Close() != first {
				t.Fatal("original close cause changed")
			}
			if !bytes.Equal(s.state.key, make([]byte, ed25519.PrivateKeySize)) {
				t.Fatal("key not cleared after joined close")
			}
			reopened, e := Open(t.Context(), root, b)
			if e != nil {
				t.Fatal("independent restored reopen", e)
			}
			if e = reopened.Close(); e != nil {
				t.Fatal(e)
			}
		})
	}
}

func TestSigningAndCopiedCloseJoinOriginalKeyUsers(t *testing.T) {
	root, _, b := importedIdentity(t)
	s, e := Open(t.Context(), root, b)
	if e != nil {
		t.Fatal(e)
	}
	copyHandle := s
	request := signingRequest(t, b)
	start := make(chan struct{})
	var users sync.WaitGroup
	for i := 0; i < 12; i++ {
		users.Add(1)
		go func() {
			defer users.Done()
			<-start
			raw, e := copyHandle.SignIssuerProfile(t.Context(), request)
			if e != nil {
				if !errors.Is(e, ErrClosed) {
					t.Error(e)
				}
				return
			}
			_, transcript, _ := request.Snapshot()
			if !ed25519.Verify(b.Signer[:], transcript, raw[len(raw)-64:]) {
				t.Error("joined signer returned bad signature")
			}
		}()
	}
	close(start)
	if e = s.Close(); e != nil {
		t.Fatal(e)
	}
	users.Wait()
	if e = copyHandle.Close(); e != nil {
		t.Fatal(e)
	}
	if raw, e := copyHandle.SignIssuerProfile(t.Context(), request); len(raw) != 0 || !errors.Is(e, ErrClosed) {
		t.Fatal("late key user", e)
	}
	if !bytes.Equal(s.state.key, make([]byte, ed25519.PrivateKeySize)) {
		t.Fatal("retained key not erased")
	}
	reopened, e := Open(t.Context(), root, b)
	if e != nil {
		t.Fatal("lease retained after join", e)
	}
	if e = reopened.Close(); e != nil {
		t.Fatal(e)
	}
}
