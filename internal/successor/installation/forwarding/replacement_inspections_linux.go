package forwarding

import (
	"bytes"
	"context"
	"crypto/ed25519"
	"crypto/sha256"
	"crypto/x509"
	"encoding/pem"
	"errors"
	"io"
	"os"
	"path/filepath"
	"syscall"
	"time"

	"github.com/dianabuilds/ardents-network/internal/successor/admission/spending"
	"github.com/dianabuilds/ardents-network/internal/successor/hosting"
	"github.com/dianabuilds/ardents-network/internal/successor/network/state"
)

// Each history owner acquires its own existing-only lease. Installation receives
// no mutable ledger, producer, Budget or runtime observation from these handles.
type replacementInspections struct {
	state          state.NodeInspection
	spend          spending.Inspection
	hosting        hosting.Inspection
	expected       state.NodeHistory
	closed         bool
	terminal       error
	emptySpend     *os.File
	emptySpendInfo os.FileInfo
	emptySpendPath string
}

func inspectReplacement(ctx context.Context, c *completedInitial) (result *replacementInspections, returned error) {
	return inspectStoppedHistories(ctx, c, nil)
}

func inspectStartRecovery(ctx context.Context, c *completedInitial, terminal *startTerminalSnapshot) (*replacementInspections, error) {
	if c == nil || c.files == nil || c.files.invocations == nil || c.files.invocations.head() == nil || c.files.invocations.head().terminal != terminal || terminal == nil || terminal.original == nil {
		return nil, ErrBinding
	}
	if err := terminal.check(); err != nil {
		return nil, err
	}
	if len(c.files.invocations.starts) != 0 {
		return inspectReplacement(ctx, c)
	}
	if c.files.invocations.snapshot != nil || len(c.files.invocations.retirements) != 0 {
		return nil, ErrBinding
	}
	return inspectStoppedHistories(ctx, c, terminal)
}

func inspectStoppedHistories(ctx context.Context, c *completedInitial, neverStarted *startTerminalSnapshot) (result *replacementInspections, returned error) {
	s := &replacementInspections{}
	defer func() {
		if returned != nil {
			s.terminal = returned
			returned = errors.Join(returned, s.close())
			result = nil
		}
	}()
	p, b := c.plan, c.binding
	n := p.Network
	pins := make(map[[32]byte]ed25519.PublicKey, len(n.Authorities))
	for _, public := range n.Authorities {
		id := sha256.Sum256(public[:])
		if _, exists := pins[id]; exists || public == [32]byte{} {
			return nil, ErrBinding
		}
		pins[id] = append(ed25519.PublicKey(nil), public[:]...)
	}
	cfg := state.Config{Root: n.Root, NetworkID: n.NetworkID, Authorities: pins, Threshold: n.Threshold,
		ClosedProfileAuthority: append(ed25519.PublicKey(nil), n.ProfileAuthority[:]...), AcceptedProfile: "ardents-route-v3",
		Clock: time.Now, ClockObservationFile: n.ClockObservationFile, LocalRoleStateRoot: p.ParticipationRoot}
	var err error
	s.state, err = state.OpenNodeInspection(ctx, cfg, p.NodeID, b.UID, b.GID)
	if err != nil {
		return nil, err
	}
	s.expected, err = s.state.Check()
	if err != nil {
		return nil, err
	}
	certificate := c.files.files[filepath.Join(b.Request.CredentialDirectory, "certificate.pem")]
	if certificate == nil {
		return nil, ErrBinding
	}
	block, rest := pem.Decode(certificate.body)
	if block == nil || block.Type != "CERTIFICATE" || len(bytes.TrimSpace(rest)) != 0 {
		return nil, ErrBinding
	}
	leaf, err := x509.ParseCertificate(block.Bytes)
	if err != nil {
		return nil, err
	}
	key, ok := leaf.PublicKey.(ed25519.PublicKey)
	if !ok || !bytes.Equal(key, s.expected.PublicKey[:]) || s.expected.NodeID != p.NodeID || s.expected.NetworkID != n.NetworkID {
		return nil, ErrBinding
	}
	if neverStarted != nil {
		// The independently completed initial birth and no-runtime child chain
		// own this empty directory. Do not create a Spending history to inspect it.
		original := c.files.dirs[p.SpendRoot]
		if original == nil || original.file == nil {
			return nil, ErrBinding
		}
		s.emptySpendPath = p.SpendRoot
		s.emptySpend, err = os.OpenFile(p.SpendRoot, os.O_RDONLY|syscall.O_DIRECTORY|syscall.O_NOFOLLOW|syscall.O_CLOEXEC, 0)
		if err != nil {
			return nil, err
		}
		s.emptySpendInfo, err = s.emptySpend.Stat()
		var fs syscall.Statfs_t
		fsErr := syscall.Fstatfs(int(s.emptySpend.Fd()), &fs)
		if err != nil || fsErr != nil || fs.Type != 0xef53 || fs.Flags&1 != 0 || !sameInputFile(original.info, s.emptySpendInfo) || !nodeAccess(s.emptySpendInfo, 0700, b.UID, b.GID, true) {
			return nil, errors.Join(ErrBinding, err, fsErr)
		}
	} else {
		s.spend, err = spending.OpenInspection(ctx, p.SpendRoot, spending.Binding{NetworkID: s.expected.NetworkID, ProfileDigest: s.expected.ProfileDigest,
			ReceiverNodeID: s.expected.NodeID, ReceiverDutyGeneration: s.expected.DutyGeneration}, b.UID, b.GID)
		if err != nil {
			return nil, err
		}
	}
	s.hosting, err = hosting.OpenInspection(ctx, p.HostingRoot, c.hostingPolicy, b.UID, b.GID)
	if err != nil {
		return nil, err
	}
	return s, s.check()
}

func (s *replacementInspections) check() error {
	if s == nil || s.closed {
		return ErrInput
	}
	if s.terminal != nil {
		return s.terminal
	}
	facts, err := s.state.Check()
	if err == nil && facts != s.expected {
		err = ErrBinding
	}
	if err == nil {
		if s.emptySpend != nil {
			err = s.checkEmptySpend()
		} else {
			err = s.spend.Check()
		}
	}
	if err == nil {
		err = s.hosting.Check()
	}
	if err == nil {
		facts, err = s.state.Check()
		if err == nil && facts != s.expected {
			err = ErrBinding
		}
	}
	if err != nil {
		s.terminal = err
	}
	return err
}

func (s *replacementInspections) checkEmptySpend() error {
	opened, err := s.emptySpend.Stat()
	named, nameErr := os.Lstat(s.emptySpendPath)
	if err != nil || nameErr != nil || !sameInputFile(s.emptySpendInfo, opened) || !sameInputFile(s.emptySpendInfo, named) {
		return errors.Join(ErrBinding, err, nameErr)
	}
	if _, err := s.emptySpend.Seek(0, io.SeekStart); err != nil {
		return err
	}
	entries, err := s.emptySpend.ReadDir(1)
	if len(entries) != 0 || err != io.EOF {
		return errors.Join(ErrBinding, err)
	}
	after, err := s.emptySpend.Stat()
	if err != nil || !sameInputFile(s.emptySpendInfo, after) {
		return errors.Join(ErrBinding, err)
	}
	return nil
}

func (s *replacementInspections) close() error {
	if s == nil {
		return nil
	}
	if !s.closed {
		s.closed = true
		// State/participation remains retained until both physical accounting
		// owners have actually joined; no result or late pass erases failures.
		s.terminal = errors.Join(s.terminal, s.hosting.Close(), s.spend.Close())
		if s.emptySpend != nil {
			s.terminal = errors.Join(s.terminal, s.emptySpend.Close())
		}
		s.terminal = errors.Join(s.terminal, s.state.Close())
	}
	return s.terminal
}
