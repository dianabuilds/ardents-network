package access

import (
	"bytes"
	"context"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/binary"
	"fmt"
	"time"

	identitycontract "ardents/api/ardents/identity/v1"
	identityprincipal "ardents/internal/identity/principal"
	"ardents/internal/storage"
)

const (
	bootstrapTicketRecordVersion byte = 1
	bootstrapTicketActive        byte = 1
	bootstrapTicketConsumed      byte = 2
	bootstrapTicketRecordBytes        = 2 + 8 + 8 + sha256.Size
)

var bootstrapTicketDomain = []byte("ardents:bootstrap-ticket:v1\x00")

type BootstrapTicket [identitycontract.BootstrapTicketBytes]byte

func (BootstrapTicket) String() string   { return "[redacted bootstrap ticket]" }
func (BootstrapTicket) GoString() string { return "[redacted bootstrap ticket]" }
func (BootstrapTicket) Format(state fmt.State, _ rune) {
	_, _ = state.Write([]byte("[redacted bootstrap ticket]"))
}
func (BootstrapTicket) MarshalJSON() ([]byte, error) {
	return []byte(`"[redacted bootstrap ticket]"`), nil
}

func (s *Service) IssueBootstrapTicket(ctx context.Context, node string) (BootstrapTicket, error) {
	if !s.bootstrapEnabled {
		return BootstrapTicket{}, ErrFeatureDisabled
	}
	if _, err := identityprincipal.Parse(node); err != nil {
		return BootstrapTicket{}, ErrInvalidArgument
	}
	now := canonicalNow(s.clock.Now())
	if now.Unix() < identitycontract.LowerTimestampUnix || now.Add(identitycontract.BootstrapTicketLifetime).Unix() >= identitycontract.UpperTimestampUnix {
		return BootstrapTicket{}, ErrInternal
	}
	var ticket BootstrapTicket
	if err := s.random(ticket[:]); err != nil {
		return BootstrapTicket{}, ErrInternal
	}
	digest := bootstrapTicketDigest(ticket)
	record := encodeBootstrapTicketRecord(bootstrapTicketActive, now, now.Add(identitycontract.BootstrapTicketLifetime), digest)
	err := s.grants.database.Update(ctx, func(tx storage.WriteTransaction) error {
		enrolled, err := nodeHasEnrollment(tx, node)
		if err != nil {
			return err
		}
		if enrolled {
			return ErrConflict
		}
		existing, found, err := tx.Get(bootstrapTicketsBucket, []byte(node))
		if err != nil {
			return err
		}
		if found {
			state, _, expires, _, decodeErr := decodeBootstrapTicketRecord(existing)
			if decodeErr != nil {
				return decodeErr
			}
			if state == bootstrapTicketConsumed || now.Before(expires) {
				return ErrConflict
			}
		}
		return tx.Put(bootstrapTicketsBucket, []byte(node), record)
	})
	if err != nil {
		return BootstrapTicket{}, mapBootstrapStoreError(err)
	}
	return ticket, nil
}

func consumeBootstrapTicket(tx storage.WriteTransaction, node string, ticket BootstrapTicket, now time.Time) error {
	enrolled, err := nodeHasEnrollment(tx, node)
	if err != nil {
		return err
	}
	if enrolled {
		return ErrConflict
	}
	record, found, err := tx.Get(bootstrapTicketsBucket, []byte(node))
	if err != nil {
		return err
	}
	if !found {
		return ErrUnauthenticated
	}
	state, issued, expires, expected, err := decodeBootstrapTicketRecord(record)
	if err != nil {
		return err
	}
	actual := bootstrapTicketDigest(ticket)
	if state != bootstrapTicketActive || now.Before(issued) || !now.Before(expires) || subtle.ConstantTimeCompare(actual[:], expected[:]) != 1 {
		return ErrUnauthenticated
	}
	return tx.Put(bootstrapTicketsBucket, []byte(node), encodeBootstrapTicketRecord(bootstrapTicketConsumed, issued, expires, expected))
}

func nodeHasEnrollment(tx storage.ReadTransaction, node string) (bool, error) {
	enrolled := false
	prefix := tuple([]byte(node))
	err := tx.ForEach(enrollmentsBucket, func(key, _ []byte) error {
		if bytes.HasPrefix(key, prefix) {
			enrolled = true
		}
		return nil
	})
	return enrolled, err
}

func bootstrapTicketDigest(ticket BootstrapTicket) [sha256.Size]byte {
	hash := sha256.New()
	_, _ = hash.Write(bootstrapTicketDomain)
	_, _ = hash.Write(ticket[:])
	var digest [sha256.Size]byte
	copy(digest[:], hash.Sum(nil))
	return digest
}

func encodeBootstrapTicketRecord(state byte, issued, expires time.Time, digest [sha256.Size]byte) []byte {
	record := make([]byte, bootstrapTicketRecordBytes)
	record[0] = bootstrapTicketRecordVersion
	record[1] = state
	binary.BigEndian.PutUint64(record[2:10], uint64(issued.Unix()))
	binary.BigEndian.PutUint64(record[10:18], uint64(expires.Unix()))
	copy(record[18:], digest[:])
	return record
}

func decodeBootstrapTicketRecord(record []byte) (byte, time.Time, time.Time, [sha256.Size]byte, error) {
	var digest [sha256.Size]byte
	if len(record) != bootstrapTicketRecordBytes || record[0] != bootstrapTicketRecordVersion || record[1] != bootstrapTicketActive && record[1] != bootstrapTicketConsumed {
		return 0, time.Time{}, time.Time{}, digest, fmt.Errorf("Bootstrap Ticket record is corrupt")
	}
	issued := time.Unix(int64(binary.BigEndian.Uint64(record[2:10])), 0).UTC()
	expires := time.Unix(int64(binary.BigEndian.Uint64(record[10:18])), 0).UTC()
	copy(digest[:], record[18:])
	if issued.Unix() < identitycontract.LowerTimestampUnix || expires.Unix() >= identitycontract.UpperTimestampUnix || !expires.Equal(issued.Add(identitycontract.BootstrapTicketLifetime)) {
		return 0, time.Time{}, time.Time{}, digest, fmt.Errorf("Bootstrap Ticket record is corrupt")
	}
	return record[1], issued, expires, digest, nil
}

func mapBootstrapStoreError(err error) error {
	if err == nil || err == ErrConflict || err == ErrUnauthenticated || err == ErrInvalidArgument {
		return err
	}
	return ErrUnavailable
}
