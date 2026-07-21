package command

import (
	"flag"
	"fmt"
	"io"
	"os"

	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/proto"
)

func ParseFileArg(name string, stderr io.Writer, args []string) (string, error) {
	flags := flag.NewFlagSet(name, flag.ContinueOnError)
	flags.SetOutput(stderr)
	var file string
	flags.StringVar(&file, "file", "", "path to JSON file or - for stdin")
	if err := flags.Parse(args); err != nil {
		return "", err
	}
	if file == "" {
		return "", fmt.Errorf("file is required")
	}
	return file, nil
}

func LoadProtoJSON(input io.Reader, path string, message proto.Message) error {
	var data []byte
	var err error
	if path == "-" {
		data, err = io.ReadAll(input)
	} else {
		data, err = os.ReadFile(path)
	}
	if err != nil {
		return fmt.Errorf("read input file: %w", err)
	}
	if err := protojson.Unmarshal(data, message); err != nil {
		return fmt.Errorf("parse input json: %w", err)
	}
	return nil
}

func FirstArg(args []string) (string, bool) {
	if len(args) == 0 || args[0] == "" {
		return "", false
	}
	return args[0], true
}
