package transfer

import "github.com/dianabuilds/ardents-network/internal/domain/connection/wire"

// RecordWrite is the state retained while the original actor writes one record.
type RecordWrite struct {
	self       *RecordWrite
	owner      *State
	data       *dataWrite
	control    *terminalWrite
	ackSerial  uint64
	generation uint64
	consumed   bool
}

// BeginRecordWrite chooses the next ACK, Data or Terminal control record.
func (s *State) BeginRecordWrite() (wire.Record, *RecordWrite, error) {
	ticket := &RecordWrite{owner: s, generation: s.Generation()}
	ticket.self = ticket
	if s.ackWritten != s.ackSerial {
		ticket.ackSerial = s.ackSerial
		return wire.Record{Acknowledgement: &wire.Acknowledgement{
			AttachmentGeneration: s.receive.generation, Offset: s.receive.next,
		}}, ticket, nil
	}
	if s.send.next != s.send.end {
		data, err := s.send.beginWrite()
		if data == nil || err != nil {
			return wire.Record{}, nil, err
		}
		ticket.data = data
		return wire.Record{Data: &data.record}, ticket, nil
	}
	control, err := s.terminal.beginWrite(s.send.next)
	if control == nil || err != nil {
		return wire.Record{}, nil, err
	}
	ticket.control = control
	return wire.Record{Terminal: control.record.Terminal, Acknowledgement: control.record.Acknowledgement}, ticket, nil
}

// FinishRecordWrite retains the result of the same original physical writer.
func (s *State) FinishRecordWrite(ticket *RecordWrite, physicalErr error) error {
	if ticket == nil || ticket.self != ticket || ticket.owner != s || ticket.generation != s.Generation() || ticket.consumed {
		return ErrIntegrity
	}
	ticket.consumed = true
	if ticket.data != nil {
		_, err := s.send.finishWrite(ticket.data, physicalErr)
		return err
	}
	if ticket.control != nil {
		_, err := s.terminal.finishWrite(ticket.control, physicalErr)
		return err
	}
	if physicalErr == nil {
		s.ackWritten = ticket.ackSerial
	}
	return physicalErr
}
