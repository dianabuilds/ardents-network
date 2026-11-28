package document

import (
	"bufio"
	"bytes"
	"context"
	"errors"
	"io"
	"path/filepath"
	"unicode"
	"unicode/utf8"

	"github.com/dianabuilds/ardents-network/internal/application/endpointstream"
	endpointclient "github.com/dianabuilds/ardents-network/internal/application/endpointstream/client"
	"github.com/dianabuilds/ardents-network/internal/application/endpointstream/targetlink"
)

// ReadLocal owns the input and output until the single document read and any
// cancellation-driven closure have joined.
func ReadLocal(ctx context.Context, socket string, input io.ReadCloser, output io.WriteCloser) (outcome error) {
	if ctx == nil || !filepath.IsAbs(socket) || input == nil || output == nil {
		return endpointstream.ErrInvalidLocalInput
	}
	var closeErr error
	closed := make(chan struct{})
	closeIO := func() error { return errors.Join(input.Close(), output.Close()) }
	stop := context.AfterFunc(ctx, func() {
		defer close(closed)
		closeErr = closeIO()
	})
	defer func() {
		if stop() {
			closeErr = closeIO()
		} else {
			<-closed
		}
		outcome = errors.Join(outcome, closeErr, ctx.Err())
	}()
	if err := ctx.Err(); err != nil {
		return err
	}
	link, err := targetlink.Read(input)
	if err != nil {
		return err
	}
	return PresentLocal(ctx, socket, link, output)
}

// WritePlainText presents only a complete bounded UTF-8 result. Its caller must
// first establish successful Service completion and joined worker cleanup.
// Control escapes are generated locally; document bytes cannot drive terminal
// commands or bidi controls. Ordinary text, including links, is inert output.
func WritePlainText(output io.Writer, body []byte) error {
	if output == nil || len(body) > MaximumBytes || !utf8.Valid(body) {
		return errors.New("document presentation is invalid")
	}
	return writeEscapedText(context.Background(), output, bytes.NewReader(body))
}

func writeEscapedText(ctx context.Context, output io.Writer, input io.Reader) error {
	writer := bufio.NewWriterSize(output, 16<<10)
	reader := bufio.NewReaderSize(input, 16<<10)
	const hex = "0123456789abcdef"
	for {
		value, err := readTextRune(ctx, reader)
		if err == io.EOF {
			return writer.Flush()
		}
		if err != nil {
			return err
		}
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
}

// PresentLocal opens one bounded local Connection to the supplied Target Link
// and presents the document after Read joins the exchange. The caller owns
// interruption and closing of the output, which can outlive the setup bound.
func PresentLocal(ctx context.Context, socket, link string, output io.Writer) (result error) {
	if ctx == nil || output == nil {
		return errors.New("document presentation is invalid")
	}
	stream, err := endpointclient.Dial(ctx, socket, endpointstream.Request{Destination: endpointstream.TargetLink, Value: link})
	if err != nil {
		return errors.Join(err, ctx.Err())
	}
	stage, err := openPresentationStage()
	if err != nil {
		return errors.Join(err, stream.Close())
	}
	defer func() { result = errors.Join(result, stage.Close()) }()
	if err := receiveDocument(ctx, stream, stage, -1); err != nil {
		return errors.Join(err, ctx.Err())
	}
	if err := validateStagedText(ctx, stage); err != nil {
		return err
	}
	return writeEscapedText(ctx, output, stage)
}

func validateStagedText(ctx context.Context, stage io.ReadSeeker) error {
	if _, err := stage.Seek(0, io.SeekStart); err != nil {
		return err
	}
	reader := bufio.NewReaderSize(stage, 16<<10)
	for {
		_, err := readTextRune(ctx, reader)
		if err == io.EOF {
			_, err := stage.Seek(0, io.SeekStart)
			return err
		}
		if err != nil {
			return err
		}
	}
}

func readTextRune(ctx context.Context, input *bufio.Reader) (rune, error) {
	if err := ctx.Err(); err != nil {
		return 0, err
	}
	value, size, err := input.ReadRune()
	if value == utf8.RuneError && size == 1 {
		return 0, errors.New("document presentation is invalid")
	}
	return value, err
}
