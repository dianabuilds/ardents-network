package document

import (
	"bufio"
	"context"
	"errors"
	"io"
	"unicode"
	"unicode/utf8"

	"github.com/dianabuilds/ardents-network/internal/application/connection"
)

// WritePlainText presents only a complete bounded UTF-8 result. Its caller must
// first establish successful Service completion and joined worker cleanup.
// Control escapes are generated locally; document bytes cannot drive terminal
// commands or bidi controls. Ordinary text, including links, is inert output.
func WritePlainText(output io.Writer, body []byte) error {
	if output == nil || len(body) > MaximumBytes || !utf8.Valid(body) {
		return errors.New("document presentation is invalid")
	}
	writer := bufio.NewWriterSize(output, 16<<10)
	const hex = "0123456789abcdef"
	for _, value := range string(body) {
		if (value < 0x20 && value != '\n' && value != '\t') ||
			(value >= 0x7f && value <= 0x9f) || unicode.Is(unicode.Bidi_Control, value) {
			escape := [6]byte{'\\', 'u', hex[(value>>12)&15], hex[(value>>8)&15], hex[(value>>4)&15], hex[value&15]}
			if _, err := writer.Write(escape[:]); err != nil {
				return err
			}
		} else if _, err := writer.WriteRune(value); err != nil {
			return err
		}
	}
	return writer.Flush()
}

// PresentLocal opens one bounded local Connection to the supplied Target Link
// and presents the document after Read joins the exchange. The caller owns
// interruption and closing of the output, which can outlive the setup bound.
func PresentLocal(ctx context.Context, socket, link string, output io.Writer) error {
	if ctx == nil || output == nil {
		return errors.New("document presentation is invalid")
	}
	bounded, cancel := context.WithTimeout(ctx, connection.ReaderTimeout)
	defer cancel()
	stream, err := connection.Dial(bounded, socket, connection.Request{Destination: connection.TargetLink, Value: link})
	if err != nil {
		return errors.Join(err, bounded.Err())
	}
	body, err := Read(bounded, stream)
	defer clear(body)
	if err != nil {
		return errors.Join(err, bounded.Err())
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	return WritePlainText(output, body)
}
