//go:build linux

package issuance

import (
	"bytes"
	"crypto"
	"crypto/ed25519"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/sha512"
	"encoding/binary"
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/cloudflare/circl/blindsign/blindrsa"
	"github.com/dianabuilds/ardents-network/internal/successor/admission"
	"github.com/dianabuilds/ardents-network/internal/successor/admission/issuerprofile"
)

func resultFixture(t *testing.T, v Inventory, class uint8, count uint16, id byte) ([]byte, admission.Facts, admission.LedgerBinding, func([]byte)) {
	t.Helper()
	authority := ed25519.NewKeyFromSeed(make([]byte, 32))
	seed := make([]byte, 32)
	seed[0] = 1
	holder := ed25519.NewKeyFromSeed(seed)
	b := admission.LedgerBinding{Network: v.Binding.Network, Issuer: v.Binding.Issuer, Authority: [32]byte(authority.Public().(ed25519.PublicKey)), Profile: [32]byte{9}, Duty: 7, Start: v.Binding.Start, End: v.Binding.End}
	for _, k := range v.Keys {
		b.Keys = append(b.Keys, issuerprofile.Key{Window: k.Window, Class: k.Class, SPKI: append([]byte(nil), k.SPKI...)})
	}
	f := admission.Facts{Network: b.Network, Issuer: b.Issuer, Authority: b.Authority, Holder: [32]byte(holder.Public().(ed25519.PublicKey)), Duty: b.Duty, DutyNotBefore: b.Start, DutyNotAfter: b.End, Now: b.Start, Class: class, Count: uint32(count)}
	permission := make([]byte, 228)
	copy(permission, b.Network[:])
	copy(permission[32:], b.Issuer[:])
	binary.BigEndian.PutUint64(permission[64:], b.Duty)
	permission[72] = class
	copy(permission[104:], f.Holder[:])
	binary.BigEndian.PutUint64(permission[136:], uint64(b.Start.Unix()))
	binary.BigEndian.PutUint64(permission[144:], uint64(b.Start.Unix()+3600))
	for i := 0; i < 3; i++ {
		binary.BigEndian.PutUint32(permission[152+4*i:], 64)
	}
	copy(permission[164:], ed25519.Sign(authority, append([]byte("ardents-issuance-permission-v1\x00"), permission[:164]...)))
	spki := v.Keys[class-1].SPKI
	public, ok := issuerprofile.ParseKey(spki)
	if !ok {
		t.Fatal("SPKI")
	}
	client, e := blindrsa.NewClient(blindrsa.SHA384PSSDeterministic, public)
	if e != nil {
		t.Fatal(e)
	}
	elements := make([]byte, 0, int(count)*259)
	var verifiers []func([]byte)
	for i := 0; i < int(count); i++ {
		message := []byte{class, id, byte(i)}
		prepared, e := client.Prepare(rand.Reader, message)
		if e != nil {
			t.Fatal(e)
		}
		blinded, state, e := client.Blind(rand.Reader, prepared)
		if e != nil {
			t.Fatal(e)
		}
		keyID := sha256.Sum256(spki)
		elements = append(elements, 0, 2, keyID[31])
		elements = append(elements, blinded...)
		verifiers = append(verifiers, func(sig []byte) {
			final, e := client.Finalize(state, sig)
			if e != nil {
				t.Fatal(e)
			}
			digest := sha512.Sum384(prepared)
			if e = rsa.VerifyPSS(public, crypto.SHA384, digest[:], final, &rsa.PSSOptions{SaltLength: 48, Hash: crypto.SHA384}); e != nil {
				t.Fatal(e)
			}
		})
	}
	request := [32]byte{id}
	raw := append([]byte("ARDIBR01"), permission...)
	raw = append(raw, request[:]...)
	raw = append(raw, class)
	raw = binary.BigEndian.AppendUint64(raw, uint64(b.Start.Unix()))
	raw = append(raw, spki...)
	raw = binary.BigEndian.AppendUint16(raw, count)
	raw = append(raw, elements...)
	transcript := append([]byte("ardents-issuance-request-v1\x00"), permission[72:104]...)
	transcript = append(transcript, request[:]...)
	transcript = append(transcript, class)
	transcript = binary.BigEndian.AppendUint64(transcript, uint64(b.Start.Unix()))
	transcript = binary.BigEndian.AppendUint16(transcript, count)
	sum := sha256.Sum256(elements)
	transcript = append(transcript, sum[:]...)
	raw = append(raw, ed25519.Sign(holder, transcript)...)
	verify := func(response []byte) {
		if len(response) != 16347 || string(response[:8]) != "ARDIOR01" || response[8] != 1 || response[9] != byte(count) {
			t.Fatal("result grammar")
		}
		for i, verify := range verifiers {
			verify(response[10+i*256 : 10+(i+1)*256])
		}
		if !bytes.Equal(response[10+int(count)*256:], make([]byte, 16347-10-int(count)*256)) {
			t.Fatal("nonzero tail")
		}
	}
	return raw, f, b, verify
}
func resultOwners(t *testing.T) (Store, Inventory, string) {
	t.Helper()
	root := filepath.Join(t.TempDir(), "keys")
	b := testBinding(1)
	if e := Initialize(t.Context(), root, b); e != nil {
		t.Fatal(e)
	}
	s, e := Open(t.Context(), root, b)
	if e != nil {
		t.Fatal(e)
	}
	t.Cleanup(func() { _ = s.Close() })
	v, e := s.Inventory()
	if e != nil {
		t.Fatal(e)
	}
	return s, v, filepath.Join(t.TempDir(), "results")
}
func TestConfirmedIssuanceRoundTrip(t *testing.T) {
	store, v, root := resultOwners(t)
	_, _, binding, _ := resultFixture(t, v, 1, 1, 1)
	ledgerRoot := filepath.Join(t.TempDir(), "admission")
	if e := admission.Initialize(ledgerRoot, binding); e != nil {
		t.Fatal(e)
	}
	l, e := admission.Open(ledgerRoot, binding)
	if e != nil {
		t.Fatal(e)
	}
	defer l.Close()
	if e := InitializeResults(t.Context(), root, store, binding); e != nil {
		t.Fatal(e)
	}
	r, e := OpenResults(t.Context(), root, store, binding)
	if e != nil {
		t.Fatal(e)
	}
	if _, _, e := r.Issue(t.Context(), admission.DebitConfirmation{}, binding.Start); !errors.Is(e, ErrInvalid) {
		t.Fatal(e)
	}
	var last []byte
	var grant admission.DebitConfirmation
	for class := uint8(1); class <= 3; class++ {
		raw, f, _, verify := resultFixture(t, v, class, 32, class)
		outcome, c := l.DebitVerified(t.Context(), raw, f, admission.Bootstrap)
		if outcome != admission.Debited {
			t.Fatal(outcome)
		}
		response, replay, e := r.Issue(t.Context(), c, f.Now)
		if e != nil || replay {
			t.Fatal(e, replay)
		}
		verify(response)
		repeated, replay, e := r.Issue(t.Context(), c, f.Now)
		if e != nil || !replay || !bytes.Equal(repeated, response) {
			t.Fatal("retry", e)
		}
		last = response
		grant = c
	}
	copyOwner := r
	if e = r.Close(); e != nil {
		t.Fatal(e)
	}
	if e = copyOwner.Close(); e != nil {
		t.Fatal(e)
	}
	r, e = OpenResults(t.Context(), root, store, binding)
	if e != nil {
		t.Fatal(e)
	}
	defer r.Close()
	restored, replay, e := r.Issue(t.Context(), grant, binding.Start)
	if e != nil || !replay || !bytes.Equal(last, restored) {
		t.Fatal("restart", e)
	}
	if _, _, e = r.Issue(t.Context(), grant, binding.End); !errors.Is(e, ErrValidity) {
		t.Fatal(e)
	}
}
func TestResultWriteFaultsAndReopen(t *testing.T) {
	store, v, _ := resultOwners(t)
	raw, f, b, _ := resultFixture(t, v, 1, 1, 1)
	ledgerRoot := filepath.Join(t.TempDir(), "admission")
	if e := admission.Initialize(ledgerRoot, b); e != nil {
		t.Fatal(e)
	}
	l, e := admission.Open(ledgerRoot, b)
	if e != nil {
		t.Fatal(e)
	}
	defer l.Close()
	outcome, c := l.DebitVerified(t.Context(), raw, f, admission.Bootstrap)
	if outcome != admission.Debited {
		t.Fatal(outcome)
	}
	for _, phase := range []string{"request-write", "response-write", "append-sync", "append-close", "floor-rename"} {
		t.Run(phase, func(t *testing.T) {
			root := filepath.Join(t.TempDir(), "results")
			if e := InitializeResults(t.Context(), root, store, b); e != nil {
				t.Fatal(e)
			}
			r, e := OpenResults(t.Context(), root, store, b)
			if e != nil {
				t.Fatal(e)
			}
			r.state.fault = func(p string) error {
				if p == phase {
					return errors.New("injected")
				}
				return nil
			}
			now := f.Now
			if phase == "floor-rename" {
				now = now.Add(time.Second)
			}
			if _, _, e = r.Issue(t.Context(), c, now); !errors.Is(e, ErrUncertain) {
				t.Fatal(e)
			}
			if _, _, e = r.Issue(t.Context(), c, now); !errors.Is(e, ErrUncertain) {
				t.Fatal("owner not terminal")
			}
			_ = r.Close()
			reopened, e := OpenResults(t.Context(), root, store, b)
			if phase == "response-write" || phase == "floor-rename" {
				if e == nil {
					_ = reopened.Close()
					t.Fatal("partial accepted")
				}
			} else {
				if e != nil {
					t.Fatal(e)
				}
				response, replay, e := reopened.Issue(t.Context(), c, f.Now)
				if e != nil || len(response) != 16347 || (phase != "request-write" && !replay) {
					t.Fatal(e, replay)
				}
				_ = reopened.Close()
			}
			again, _ := l.DebitVerified(t.Context(), raw, f, admission.Bootstrap)
			if again != admission.AlreadyDebited {
				t.Fatal("refunded", again)
			}
		})
	}
}
func TestResultTampering(t *testing.T) {
	store, v, _ := resultOwners(t)
	_, _, b, _ := resultFixture(t, v, 1, 1, 1)
	for _, name := range []string{"results.pin", "results.floor", "results.journal", "results.pending", "extra"} {
		t.Run(name, func(t *testing.T) {
			root := filepath.Join(t.TempDir(), "results")
			if e := InitializeResults(t.Context(), root, store, b); e != nil {
				t.Fatal(e)
			}
			if e := os.WriteFile(filepath.Join(root, name), []byte("corrupt"), 0600); e != nil {
				t.Fatal(e)
			}
			if r, e := OpenResults(t.Context(), root, store, b); e == nil {
				_ = r.Close()
				t.Fatal("corrupt accepted")
			}
		})
	}
}

func TestResultPermissionsSubstitutionAndBusy(t *testing.T) {
	store, v, _ := resultOwners(t)
	_, _, b, _ := resultFixture(t, v, 1, 1, 1)
	for _, attack := range []string{"mode", "root-mode", "symlink", "hardlink", "missing", "binding"} {
		t.Run(attack, func(t *testing.T) {
			root := filepath.Join(t.TempDir(), "results")
			if e := InitializeResults(t.Context(), root, store, b); e != nil {
				t.Fatal(e)
			}
			name := filepath.Join(root, "results.journal")
			switch attack {
			case "mode":
				if e := os.Chmod(name, 0644); e != nil {
					t.Fatal(e)
				}
			case "root-mode":
				if e := os.Chmod(root, 0755); e != nil {
					t.Fatal(e)
				}
			case "symlink":
				if e := os.Rename(name, name+".saved"); e != nil {
					t.Fatal(e)
				}
				if e := os.Symlink(name+".saved", name); e != nil {
					t.Fatal(e)
				}
			case "hardlink":
				if e := os.Link(name, filepath.Join(filepath.Dir(root), "linked")); e != nil {
					t.Fatal(e)
				}
			case "missing":
				if e := os.Remove(name); e != nil {
					t.Fatal(e)
				}
			case "binding":
				b2 := b
				b2.Profile[0]++
				if r, e := OpenResults(t.Context(), root, store, b2); e == nil {
					_ = r.Close()
					t.Fatal("binding accepted")
				}
				return
			}
			if r, e := OpenResults(t.Context(), root, store, b); e == nil {
				_ = r.Close()
				t.Fatal("attack accepted")
			}
		})
	}
	root := filepath.Join(t.TempDir(), "results")
	if e := InitializeResults(t.Context(), root, store, b); e != nil {
		t.Fatal(e)
	}
	r, e := OpenResults(t.Context(), root, store, b)
	if e != nil {
		t.Fatal(e)
	}
	if _, e := OpenResults(t.Context(), root, store, b); !errors.Is(e, ErrBusy) {
		t.Fatal(e)
	}
	path := filepath.Join(root, "results.journal")
	saved, e := os.ReadFile(path)
	if e != nil {
		t.Fatal(e)
	}
	if e = os.Rename(path, path+".old"); e != nil {
		t.Fatal(e)
	}
	if e = os.WriteFile(path, saved, 0600); e != nil {
		t.Fatal(e)
	}
	if e = r.Close(); e == nil {
		t.Fatal("cleanup lost substitution")
	}
	if r.Close() == nil {
		t.Fatal("close retry lost error")
	}
}
