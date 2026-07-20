package cli

import (
	"flag"
	"fmt"
	"io"
	"os"

	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/proto"
)

func parseFileArg(name string, stderr io.Writer, args []string) (string, error) {
	fs := flag.NewFlagSet(name, flag.ContinueOnError)
	fs.SetOutput(stderr)
	var file string
	fs.StringVar(&file, "file", "", "path to JSON file or - for stdin")
	if err := fs.Parse(args); err != nil {
		return "", err
	}
	return file, requireValue("file", file)
}

func loadProtoJSON(path string, msg proto.Message) error {
	var data []byte
	var err error
	if path == "-" {
		data, err = ioReadAll(os.Stdin)
	} else {
		data, err = os.ReadFile(path)
	}
	if err != nil {
		return fmt.Errorf("read input file: %w", err)
	}
	if err := protojson.Unmarshal(data, msg); err != nil {
		return fmt.Errorf("parse input json: %w", err)
	}
	return nil
}

func firstArg(args []string) (string, bool) {
	if len(args) == 0 || args[0] == "" {
		return "", false
	}
	return args[0], true
}
