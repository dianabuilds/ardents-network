package recordio

import (
	"bytes"
	"errors"
	"io"

	"github.com/dianabuilds/ardents-network/internal/domain/connection/wire"
)

// WriteInitialRequest sends Challenge and Continuity in one physical write.
func WriteInitialRequest(writer io.Writer, challenge wire.Challenge, continuity wire.Continuity) error {
	var flight bytes.Buffer
	if err := wire.Write(&flight, wire.Record{Challenge: &challenge}); err != nil {
		return err
	}
	if err := wire.Write(&flight, wire.Record{Continuity: &continuity}); err != nil {
		return err
	}
	n, err := writer.Write(flight.Bytes())
	if err != nil || n != flight.Len() {
		return errors.Join(err, io.ErrShortWrite)
	}
	return nil
}
