package session

import (
	"context"
	"errors"
	"net"
	"reflect"
	"testing"

	"github.com/dianabuilds/ardents-network/internal/successor/admission/stock"
	"github.com/dianabuilds/ardents-network/internal/successor/endpoint/funding"
	executionruntime "github.com/dianabuilds/ardents-network/internal/successor/execution/runtime"
)

func TestPreparationAndLiveOperationKeepDifferentPurposes(t *testing.T) {
	prepared := map[action]bool{actionRequest: true, actionImport: true, actionBootstrapOpen: true, actionBootstrapClose: true, actionIssue: true, actionStatus: true, actionClose: true}
	excluded := map[action]bool{actionJoinPrefixOpen: true, actionJoinOpen: true, actionJoinClose: true, actionRecipient: true, actionRegistrationOpen: true, actionRegistrationWithdraw: true, actionRegistrationClose: true, actionDescriptorPublish: true}
	for a := actionRequest; a <= actionDescriptorPublish; a++ {
		if preparationAllows(a) != prepared[a] {
			t.Fatalf("preparation purpose %d", a)
		}
		if operationAllows(a) == excluded[a] {
			t.Fatalf("live operation purpose %d", a)
		}
	}
	if preparationAllows(action(255)) || operationAllows(action(255)) {
		t.Fatal("unknown purpose accepted")
	}
}

func TestUnavailablePermissionRefusesBeforeHolderTransfer(t *testing.T) {
	// An unopened Stock supplies no authority; constructors must refuse the sealed
	// missing Execution provenance before any Stock operation or ownership transfer.
	holder := new(stock.Owner)
	if s, err := OpenPreparation(holder, funding.Authority{}, Config{}, new(executionruntime.Preparation)); s != nil || !errors.Is(err, ErrPermission) {
		t.Fatal(s, err)
	}
	if s, err := OpenOperation(holder, funding.Authority{}, Config{}, new(executionruntime.Operation)); s != nil || !errors.Is(err, ErrPermission) {
		t.Fatal(s, err)
	}
	for _, p := range []executionruntime.Permission{new(executionruntime.Preparation), new(executionruntime.Operation)} {
		s := &Session{permission: p}
		if err := s.OpenPrefix(t.Context()); !errors.Is(err, ErrPermission) {
			t.Fatal("missing permission reached prefix", err)
		}
		if err := s.OpenPublication(t.Context()); !errors.Is(err, ErrPermission) {
			t.Fatal("missing permission reached Publication", err)
		}
		if _, _, err := s.Request(t.Context(), [3]uint32{1}); !errors.Is(err, ErrPermission) {
			t.Fatal("missing permission reached Stock", err)
		}
	}
}

type heldClose struct {
	net.Conn
	entered, release chan struct{}
	cause            error
}

func (c *heldClose) Close() error {
	close(c.entered)
	<-c.release
	return errors.Join(c.Conn.Close(), c.cause)
}

func TestSessionCloseWaitsForPhysicalChildAndRetainsEveryFailure(t *testing.T) {
	left, right := net.Pipe()
	defer right.Close()
	childFailure, prefixFailure := errors.New("physical child close"), errors.New("prefix close")
	child := &heldClose{Conn: left, entered: make(chan struct{}), release: make(chan struct{}), cause: childFailure}
	var order []string
	prefixReached := make(chan struct{}, 1)
	s := &Session{joined: child, prefix: Prefix{close: func() error { order = append(order, "prefix"); prefixReached <- struct{}{}; return prefixFailure }}, routeContext: JoinContext{close: func() error { order = append(order, "context"); return nil }}}
	done := make(chan error, 1)
	go func() { done <- s.Close() }()
	<-child.entered
	select {
	case <-prefixReached:
		t.Fatal("parent returned before child physical join")
	default:
	}
	close(child.release)
	err := <-done
	if !errors.Is(err, childFailure) || !errors.Is(err, prefixFailure) {
		t.Fatal("cleanup cause lost", err)
	}
	if again := s.Close(); again != err {
		t.Fatal("repeat Close changed original result", again, err)
	}
	if !reflect.DeepEqual(order, []string{"prefix", "context"}) {
		t.Fatal("duplicate or reordered parent close", order)
	}
	if _, err := s.Status(context.Background()); err == nil {
		t.Fatal("closed conversation accepted operation")
	}
}
