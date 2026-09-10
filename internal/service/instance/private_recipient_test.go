package instance

import (
	"crypto/ed25519"
	"crypto/rand"
	"errors"
	"testing"
	"time"
)

func TestPrivateRecipientRequiresConsumedInstanceAndRetainsRevisionFloor(t *testing.T) {
	now := time.Date(2030, 4, 5, 6, 7, 8, 0, time.UTC)
	root, err := Initialize(InitializeConfig{Root: instanceFixtureRoot(t), NetworkID: [32]byte{21}, NotBefore: now, NotAfter: now.Add(time.Hour)})
	if err != nil {
		t.Fatal(err)
	}
	defer root.Close()
	_, authority, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	defer clear(authority)
	if _, err := root.Accept(issuedResponse(t, root, authority, 1)); err != nil {
		t.Fatal(err)
	}
	binding, err := root.OpenBinding(0)
	if err != nil {
		t.Fatal(err)
	}
	end := now.Add(time.Minute)
	if _, err := binding.NewPrivateRecipient(1, now, end); !errors.Is(err, ErrUnavailable) {
		t.Fatalf("unconsumed Instance issued key: %v", err)
	}
	if err := binding.CommitPublished(1); err != nil {
		t.Fatal(err)
	}
	first, err := binding.NewPrivateRecipient(1, now, end)
	if err != nil {
		t.Fatal(err)
	}
	public := first.Public(now)
	if public == [32]byte{} || public == binding.IntroductionPublic() {
		t.Fatal("private recipient reused legacy key")
	}
	overlap, err := binding.NewPrivateRecipient(2, now, end)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := binding.NewPrivateRecipient(3, now, end); err == nil {
		t.Fatal("unbounded third simultaneous recipient")
	}
	if err := overlap.Close(); err != nil {
		t.Fatal(err)
	}
	material := first.private
	if err := first.Close(); err != nil {
		t.Fatal(err)
	}
	for _, value := range material {
		if value != 0 {
			t.Fatal("recipient scalar retained after close")
		}
	}
	if _, err := binding.NewPrivateRecipient(1, now, end); err == nil {
		t.Fatal("retired revision reused")
	}
	second, err := binding.NewPrivateRecipient(3, now, end)
	if err != nil {
		t.Fatal(err)
	}
	if second.Public(now) == public {
		t.Fatal("new revision reused recipient")
	}
	if second.Public(end) != [32]byte{} || second.private != nil {
		t.Fatal("expired recipient remained live")
	}
	third, err := binding.NewPrivateRecipient(4, now, end)
	if err != nil {
		t.Fatal(err)
	}
	if err := binding.Withdraw(); err != nil {
		t.Fatal(err)
	}
	if third.Public(now) != [32]byte{} || third.private != nil {
		t.Fatal("withdrawn Instance retained recipient")
	}
	if _, err := binding.NewPrivateRecipient(5, now, end); err == nil {
		t.Fatal("withdrawn Instance issued recipient")
	}
}

func TestPrivateRecipientPredecessorCannotExtendAndErasesIndependently(t *testing.T) {
	now := time.Date(2030, 4, 5, 6, 7, 8, 0, time.UTC)
	root, err := Initialize(InitializeConfig{Root: instanceFixtureRoot(t), NetworkID: [32]byte{21}, NotBefore: now, NotAfter: now.Add(time.Hour)})
	if err != nil {
		t.Fatal(err)
	}
	defer root.Close()
	_, authority, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	defer clear(authority)
	if _, err := root.Accept(issuedResponse(t, root, authority, 1)); err != nil {
		t.Fatal(err)
	}
	binding, err := root.OpenBinding(0)
	if err != nil {
		t.Fatal(err)
	}
	if err := binding.CommitPublished(1); err != nil {
		t.Fatal(err)
	}
	first, err := binding.NewPrivateRecipient(1, now, now.Add(600*time.Second))
	if err != nil {
		t.Fatal(err)
	}
	second, err := binding.NewPrivateRecipient(2, now, now.Add(600*time.Second))
	if err != nil {
		t.Fatal(err)
	}
	if err := first.RetainPredecessor(now, now.Add(61*time.Second)); err == nil {
		t.Fatal("predecessor exceeded 60 seconds")
	}
	cutoff := now.Add(60 * time.Second)
	if err := first.RetainPredecessor(now, cutoff); err != nil {
		t.Fatal(err)
	}
	if err := first.RetainPredecessor(now.Add(time.Second), cutoff.Add(time.Second)); err == nil {
		t.Fatal("repeated replacement extended predecessor")
	}
	material := first.private
	if first.Public(cutoff.Add(-time.Nanosecond)) == [32]byte{} {
		t.Fatal("predecessor retired before cutoff")
	}
	if first.Public(cutoff) != [32]byte{} || second.Public(cutoff) == [32]byte{} {
		t.Fatal("cutoff did not erase only the predecessor")
	}
	for _, value := range material {
		if value != 0 {
			t.Fatal("retired private scalar retained")
		}
	}
	if err := first.RetainPredecessor(cutoff, cutoff.Add(time.Second)); err == nil {
		t.Fatal("retired recipient revived")
	}
}
