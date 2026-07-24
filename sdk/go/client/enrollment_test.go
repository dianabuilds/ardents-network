package client

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"testing"

	identitycontract "ardents/api/ardents/identity/v1"

	"github.com/stretchr/testify/require"
)

func TestApplicationEnrollmentTicketParsingIsCanonicalAndRedacted(t *testing.T) {
	raw := make([]byte, identitycontract.ApplicationEnrollmentTicketBytes)
	for index := range raw {
		raw[index] = byte(index + 1)
	}
	encoded := base64.RawURLEncoding.EncodeToString(raw)
	ticket, err := ParseApplicationEnrollmentTicket(encoded)
	require.NoError(t, err)
	formatted := fmt.Sprintf("%v %#v %x", ticket, ticket, ticket)
	require.NotContains(t, formatted, encoded)
	require.NotContains(t, formatted, fmt.Sprintf("%x", raw))
	jsonRaw, err := json.Marshal(ticket)
	require.NoError(t, err)
	require.NotContains(t, string(jsonRaw), encoded)

	for _, invalid := range []string{"", " " + encoded, encoded + "=", base64.RawURLEncoding.EncodeToString(make([]byte, len(raw)))} {
		_, err = ParseApplicationEnrollmentTicket(invalid)
		require.Error(t, err)
	}
}

func TestApplicationEnrollmentRejectsPaddedNodePrincipalBeforeTransportSetup(t *testing.T) {
	var value [identitycontract.ApplicationEnrollmentTicketBytes]byte
	value[0] = 1
	ticket := ApplicationEnrollmentTicket{value: value}
	for _, node := range []string{" " + canonicalNodePrincipal, canonicalNodePrincipal + "\n"} {
		_, err := EnrollApplication(context.Background(), EnrollmentConfig{
			SocketPath:    filepath.Join(t.TempDir(), "application.sock"),
			NodePrincipal: node,
			Ticket:        ticket,
			Signer:        &clientSignerStub{},
		})
		require.ErrorContains(t, err, "enrollment configuration is invalid")
	}
}

func TestEnrollApplicationFromFileRejectsUnsafeTicketFilesBeforeEnrollment(t *testing.T) {
	raw := make([]byte, identitycontract.ApplicationEnrollmentTicketBytes)
	for index := range raw {
		raw[index] = byte(index + 1)
	}
	encoded := base64.RawURLEncoding.EncodeToString(raw)
	dir := t.TempDir()

	tests := []struct {
		name    string
		content string
		mode    os.FileMode
	}{
		{name: "padded", content: " " + encoded, mode: 0o600},
		{name: "trailing newline", content: encoded + "\n", mode: 0o600},
		{name: "oversized", content: encoded + encoded, mode: 0o600},
	}
	if runtime.GOOS != "windows" {
		tests = append(tests, struct {
			name    string
			content string
			mode    os.FileMode
		}{name: "group readable", content: encoded, mode: 0o640})
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			path := filepath.Join(dir, test.name)
			require.NoError(t, os.WriteFile(path, []byte(test.content), test.mode))
			_, err := openApplicationEnrollmentTicket(path)
			require.ErrorContains(t, err, "Application enrollment ticket file is invalid")
			require.FileExists(t, path)
		})
	}
}

func TestEnrollApplicationFromFileRejectsRelativeAndNonRegularTicketPaths(t *testing.T) {
	_, err := openApplicationEnrollmentTicket("application-enrollment-ticket")
	require.ErrorContains(t, err, "Application enrollment ticket file is invalid")

	dirPath := t.TempDir()
	_, err = openApplicationEnrollmentTicket(dirPath)
	require.ErrorContains(t, err, "Application enrollment ticket file is invalid")
}

func TestEnrollApplicationFromFileRejectsSymlinkTicket(t *testing.T) {
	raw := make([]byte, identitycontract.ApplicationEnrollmentTicketBytes)
	for index := range raw {
		raw[index] = byte(index + 1)
	}
	dir := t.TempDir()
	target := filepath.Join(dir, "target")
	link := filepath.Join(dir, "ticket")
	require.NoError(t, os.WriteFile(target, []byte(base64.RawURLEncoding.EncodeToString(raw)), 0o600))
	if err := os.Symlink(target, link); err != nil {
		t.Skipf("symlinks are unavailable: %v", err)
	}

	_, err := openApplicationEnrollmentTicket(link)
	require.ErrorContains(t, err, "Application enrollment ticket file is invalid")
	require.FileExists(t, target)
}

func TestEnrollApplicationFromFileRetainsTicketWhenEnrollmentDoesNotCommit(t *testing.T) {
	raw := make([]byte, identitycontract.ApplicationEnrollmentTicketBytes)
	for index := range raw {
		raw[index] = byte(index + 1)
	}
	path := filepath.Join(t.TempDir(), "application-enrollment-ticket")
	require.NoError(t, os.WriteFile(path, []byte(base64.RawURLEncoding.EncodeToString(raw)), 0o600))

	_, err := EnrollApplicationFromFile(context.Background(), EnrollmentFileConfig{
		SocketPath:    filepath.Join(t.TempDir(), "missing.sock"),
		NodePrincipal: canonicalNodePrincipal,
		TicketPath:    path,
		Signer:        &clientSignerStub{},
	})
	require.Error(t, err)
	require.FileExists(t, path)
}

func TestEnrollmentFileSeamRemovesTicketOnlyAfterValidatedSuccess(t *testing.T) {
	raw := make([]byte, identitycontract.ApplicationEnrollmentTicketBytes)
	for index := range raw {
		raw[index] = byte(index + 1)
	}
	path := filepath.Join(t.TempDir(), "application-enrollment-ticket")
	require.NoError(t, os.WriteFile(path, []byte(base64.RawURLEncoding.EncodeToString(raw)), 0o600))
	want := EnrollmentResult{Principal: canonicalNodePrincipal, CredentialID: "credential", GrantID: "grant"}
	calls := 0

	got, err := enrollApplicationFromFile(context.Background(), EnrollmentFileConfig{
		TicketPath: path,
	}, func(context.Context, EnrollmentConfig) (EnrollmentResult, error) {
		calls++
		return want, nil
	}, (*applicationEnrollmentTicketFile).remove)
	require.NoError(t, err)
	require.Equal(t, want, got)
	require.Equal(t, 1, calls)
	require.NoFileExists(t, path)
}

func TestEnrollmentFileSeamReportsCommittedEnrollmentWhenCleanupFails(t *testing.T) {
	raw := make([]byte, identitycontract.ApplicationEnrollmentTicketBytes)
	for index := range raw {
		raw[index] = byte(index + 1)
	}
	encoded := base64.RawURLEncoding.EncodeToString(raw)
	path := filepath.Join(t.TempDir(), "application-enrollment-ticket")
	require.NoError(t, os.WriteFile(path, []byte(encoded), 0o600))
	want := EnrollmentResult{Principal: canonicalNodePrincipal, CredentialID: "credential", GrantID: "grant"}
	cleanupFailure := errors.New("injected cleanup failure")

	got, err := enrollApplicationFromFile(context.Background(), EnrollmentFileConfig{
		TicketPath: path,
	}, func(context.Context, EnrollmentConfig) (EnrollmentResult, error) {
		return want, nil
	}, func(*applicationEnrollmentTicketFile) (EnrollmentTicketFileState, error) {
		return EnrollmentTicketFileRetained, cleanupFailure
	})
	require.Equal(t, want, got)
	var cleanupErr *EnrollmentFileCleanupError
	require.ErrorAs(t, err, &cleanupErr)
	require.Equal(t, want, cleanupErr.Result)
	require.Equal(t, EnrollmentTicketFileRetained, cleanupErr.TicketFileState)
	require.ErrorIs(t, err, cleanupFailure)
	require.FileExists(t, path)
}

func TestEnrollmentFileCleanupRefusesAReplacedPath(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Windows prevents replacing an open ticket file")
	}
	raw := make([]byte, identitycontract.ApplicationEnrollmentTicketBytes)
	for index := range raw {
		raw[index] = byte(index + 1)
	}
	encoded := base64.RawURLEncoding.EncodeToString(raw)
	dir := t.TempDir()
	path := filepath.Join(dir, "application-enrollment-ticket")
	original := filepath.Join(dir, "original-ticket")
	require.NoError(t, os.WriteFile(path, []byte(encoded), 0o600))
	ticketFile, err := openApplicationEnrollmentTicket(path)
	require.NoError(t, err)
	defer ticketFile.close()
	require.NoError(t, os.Rename(path, original))
	require.NoError(t, os.WriteFile(path, []byte(encoded), 0o600))

	state, err := ticketFile.remove()
	require.Error(t, err)
	require.Equal(t, EnrollmentTicketFileUnknown, state)
	require.FileExists(t, path)
	require.FileExists(t, original)
}

func TestEnrollmentFileCleanupRemainsBoundToTheOpenedParentDirectory(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Application ticket cleanup uses the Unix Application listener")
	}
	raw := make([]byte, identitycontract.ApplicationEnrollmentTicketBytes)
	for index := range raw {
		raw[index] = byte(index + 1)
	}
	encoded := base64.RawURLEncoding.EncodeToString(raw)
	base := t.TempDir()
	dir := filepath.Join(base, "handoff")
	movedDir := filepath.Join(base, "moved-handoff")
	require.NoError(t, os.Mkdir(dir, 0o700))
	path := filepath.Join(dir, "application-enrollment-ticket")
	require.NoError(t, os.WriteFile(path, []byte(encoded), 0o600))
	ticketFile, err := openApplicationEnrollmentTicket(path)
	require.NoError(t, err)
	defer ticketFile.close()

	require.NoError(t, os.Rename(dir, movedDir))
	require.NoError(t, os.Mkdir(dir, 0o700))
	replacement := filepath.Join(dir, "application-enrollment-ticket")
	require.NoError(t, os.WriteFile(replacement, []byte(encoded), 0o600))

	state, err := ticketFile.remove()
	require.NoError(t, err)
	require.Equal(t, EnrollmentTicketFileRetired, state)
	require.FileExists(t, replacement)
	require.NoFileExists(t, filepath.Join(movedDir, "application-enrollment-ticket"))
}
