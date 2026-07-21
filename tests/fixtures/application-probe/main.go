package main

import (
	"ardents/sdk/go/client"
	"context"
	"fmt"
	"os"
)

func main() {
	if len(os.Args) != 3 {
		_, _ = fmt.Fprintln(os.Stderr, "usage: application-probe SOCKET TOKEN_FILE")
		os.Exit(2)
	}
	credential, err := client.FileCredential(os.Args[2])
	if err != nil {
		fail(err)
	}
	application, err := client.New(client.Config{SocketPath: os.Args[1], Credential: credential})
	if err != nil {
		fail(err)
	}
	reference, err := application.Content.Put(context.Background(), []byte("native application probe"))
	if err != nil {
		fail(err)
	}
	payload, err := application.Content.Get(context.Background(), reference)
	if err != nil {
		fail(err)
	}
	if string(payload) != "native application probe" {
		fail(fmt.Errorf("unexpected content payload"))
	}
}

func fail(err error) {
	_, _ = fmt.Fprintln(os.Stderr, err)
	os.Exit(1)
}
