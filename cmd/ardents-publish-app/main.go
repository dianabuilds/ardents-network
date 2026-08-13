package main

import (
	"errors"
	"fmt"
	"io"
	"net"
	"os"
	"time"
)

func main() {
	if err := run(os.Args[1:], os.Stdout); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func run(arguments []string, output io.Writer) error {
	if len(arguments) != 2 || arguments[0] != "publish" || arguments[1] == "" {
		return errors.New("usage: ardents-publish-app publish <administration-socket>")
	}
	connection, err := net.DialTimeout("unix", arguments[1], 5*time.Second)
	if err != nil {
		return err
	}
	defer connection.Close()
	if err := publish(connection); err != nil {
		return err
	}
	_, err = fmt.Fprintln(output, "published")
	return err
}

func publish(connection io.ReadWriter) error {
	if _, err := connection.Write([]byte("publish\n")); err != nil {
		return err
	}
	response := make([]byte, 10)
	if _, err := io.ReadFull(connection, response); err != nil {
		return err
	}
	if string(response) != "published\n" {
		return errors.New("service administration rejected publication")
	}
	return nil
}
