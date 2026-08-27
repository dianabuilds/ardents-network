package main

import (
	"bytes"
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/dianabuilds/ardents-network/internal/release/custody"
)

func TestRunRejectsInvalidArgumentsBeforeSecretInput(t *testing.T) {
	err := run(context.Background(), []string{"initialize"}, &bytes.Buffer{}, unreadInput{})
	if err == nil || !strings.Contains(err.Error(), "arguments") {
		t.Fatalf("run = %v", err)
	}
}

func TestRunInitializesRecordAndRendersOnlyReceipt(t *testing.T) {
	root := t.TempDir()
	var output bytes.Buffer
	password := []byte("release-custody-password")
	err := run(context.Background(), []string{"initialize", "--root", root}, &output, &fixedInput{values: [][]byte{password, password}})
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(output.String(), string(password)) || !strings.Contains(output.String(), "ardents-release-custody-receipt-v1") {
		t.Fatalf("receipt = %q", output.String())
	}
}

func TestRunInspectsExistingRecordWithOneSecret(t *testing.T) {
	root := t.TempDir()
	password := []byte("release-custody-password")
	if err := run(context.Background(), []string{"initialize", "--root", root}, &bytes.Buffer{}, &fixedInput{values: [][]byte{password, password}}); err != nil {
		t.Fatal(err)
	}
	var output bytes.Buffer
	if err := run(context.Background(), []string{"inspect", "--root", root}, &output, &fixedInput{values: [][]byte{password}}); err != nil {
		t.Fatal(err)
	}
	if strings.Contains(output.String(), string(password)) || !strings.Contains(output.String(), "ardents-release-custody-receipt-v1") {
		t.Fatalf("receipt = %q", output.String())
	}
}

type unreadInput struct{}

func (unreadInput) ReadSecret(context.Context, custody.Prompt) ([]byte, error) {
	return nil, errors.New("secret read should not occur")
}

type fixedInput struct {
	values [][]byte
	next   int
}

func (input *fixedInput) ReadSecret(_ context.Context, _ custody.Prompt) ([]byte, error) {
	if input.next >= len(input.values) {
		return nil, errors.New("unexpected secret request")
	}
	value := append([]byte(nil), input.values[input.next]...)
	input.next++
	return value, nil
}
