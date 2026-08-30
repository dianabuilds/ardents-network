package main

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net"
	"os"
	"time"

	"github.com/dianabuilds/ardents-network/internal/endpoint"
)

func runHeadlessOpen(ctx context.Context, socket, serviceLink, inputPath, outputPath string, output io.Writer) error {
	input, err := os.Open(inputPath)
	if err != nil {
		return err
	}
	defer input.Close()
	result, err := os.OpenFile(outputPath, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
	if err != nil {
		return err
	}
	keep := false
	defer func() {
		_ = result.Close()
		if !keep {
			_ = os.Remove(outputPath)
		}
	}()
	application, err := endpoint.DialLocalApplication(ctx, socket, serviceLink)
	if err != nil {
		return err
	}
	defer application.Close()
	response := make(chan error, 1)
	go func() {
		_, copyErr := io.Copy(result, application)
		response <- copyErr
	}()
	if _, err := io.Copy(application, input); err != nil {
		return err
	}
	if err := application.CloseInput(); err != nil {
		return err
	}
	if err := <-response; err != nil {
		return err
	}
	outcome, open := <-application.Done()
	if !open || outcome.Class == "" {
		return errors.New("headless Application ended without a terminal outcome")
	}
	if err := result.Sync(); err != nil {
		return err
	}
	keep = true
	return json.NewEncoder(output).Encode(struct {
		Kind, Class, Reason string
	}{Kind: "headless-open-complete", Class: outcome.Class, Reason: outcome.Reason})
}

func runHeadlessAdministration(ctx context.Context, operation, socket string, output io.Writer) error {
	connection, err := (&net.Dialer{}).DialContext(ctx, "unix", socket)
	if err != nil {
		return err
	}
	defer connection.Close()
	if deadline, available := ctx.Deadline(); available {
		_ = connection.SetDeadline(deadline)
	} else {
		_ = connection.SetDeadline(time.Now().Add(15 * time.Second))
	}
	if _, err := io.WriteString(connection, operation+"\n"); err != nil {
		return err
	}
	unix, ok := connection.(*net.UnixConn)
	if !ok {
		return errors.New("headless Service Administration attachment is not a Unix connection")
	}
	if err := unix.CloseWrite(); err != nil {
		return err
	}
	want := map[string]string{"publish": "published\n", "withdraw": "withdrawn\n"}[operation]
	response, err := io.ReadAll(io.LimitReader(connection, 64))
	if want == "" || err != nil || string(response) != want {
		return errors.Join(err, errors.New("headless Service Administration request failed"))
	}
	return json.NewEncoder(output).Encode(map[string]string{"kind": "headless-service-" + string(response[:len(response)-1])})
}
