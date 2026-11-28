package localread

import (
	"bufio"
	"context"
	"errors"
	"io"
	"unicode"
	"unicode/utf8"
)

// presentStaged validates every byte before writing terminal output.
func presentStaged(ctx context.Context, output io.Writer, input io.ReadSeeker) error {
	if output == nil {
		return errors.New("document presentation is invalid")
	}
	if err := validateStagedText(ctx, input); err != nil {
		return err
	}
	return writeEscapedText(ctx, output, input)
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
