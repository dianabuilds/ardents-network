package main

import (
	"crypto/ed25519"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
)

type publicReceipt struct {
	Schema string `json:"schema"`
	Public string `json:"instance_public"`
}

func main() {
	if err := run(os.Args[1:], os.Stdout); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func run(arguments []string, output io.Writer) error {
	if len(arguments) != 2 || arguments[0] != "generate" || arguments[1] == "" {
		return errors.New("usage: ardents-instance-fixture generate <new-private-key-path>")
	}
	public, private, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		return err
	}
	defer erase(private)
	file, err := os.OpenFile(arguments[1], os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
	if err != nil {
		return err
	}
	_, writeErr := file.WriteString(hex.EncodeToString(private))
	closeErr := file.Close()
	if writeErr != nil || closeErr != nil {
		return errors.Join(writeErr, closeErr)
	}
	return json.NewEncoder(output).Encode(publicReceipt{Schema: "ardents-h3-instance-host-v1",
		Public: hex.EncodeToString(public)})
}

func erase(value []byte) {
	for index := range value {
		value[index] = 0
	}
}
