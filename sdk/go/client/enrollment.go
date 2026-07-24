package client

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	identitycontract "ardents/api/ardents/identity/v1"
	sdkidentity "ardents/sdk/go/identity"
	"ardents/sdk/go/internal/adapter"
)

// EnrollmentSigner exposes only the two typed operations required for
// Application enrollment. It is intentionally not a generic signing oracle.
type EnrollmentSigner interface {
	Principal(context.Context) (string, error)
	Credential(context.Context) (*sdkidentity.Artifact, error)
	SignEnrollmentChallenge(context.Context, sdkidentity.Challenge) ([]byte, error)
}

type ApplicationEnrollmentTicket struct {
	value [identitycontract.ApplicationEnrollmentTicketBytes]byte
}

func ParseApplicationEnrollmentTicket(encoded string) (ApplicationEnrollmentTicket, error) {
	var result ApplicationEnrollmentTicket
	raw, ok := identitycontract.DecodeApplicationEnrollmentTicket(encoded)
	if !ok {
		return result, fmt.Errorf("Application enrollment ticket is invalid")
	}
	result.value = raw
	return result, nil
}

func (ApplicationEnrollmentTicket) String() string { return "[redacted Application enrollment ticket]" }
func (ApplicationEnrollmentTicket) GoString() string {
	return "[redacted Application enrollment ticket]"
}
func (ApplicationEnrollmentTicket) Format(state fmt.State, _ rune) {
	_, _ = state.Write([]byte("[redacted Application enrollment ticket]"))
}
func (ApplicationEnrollmentTicket) MarshalJSON() ([]byte, error) {
	return json.Marshal("[redacted Application enrollment ticket]")
}

type EnrollmentConfig struct {
	SocketPath    string
	NodePrincipal string
	Ticket        ApplicationEnrollmentTicket
	Signer        EnrollmentSigner
	HTTPClient    *http.Client
}

type EnrollmentFileConfig struct {
	SocketPath    string
	NodePrincipal string
	TicketPath    string
	Signer        EnrollmentSigner
	HTTPClient    *http.Client
}

type EnrollmentResult struct {
	Principal      string
	CredentialID   string
	GrantID        string
	GrantExpiresAt time.Time
}

type EnrollmentTicketFileState string

const (
	EnrollmentTicketFileRetained EnrollmentTicketFileState = "retained"
	EnrollmentTicketFileRetired  EnrollmentTicketFileState = "retired"
	EnrollmentTicketFileUnknown  EnrollmentTicketFileState = "unknown"
)

// EnrollmentFileCleanupError means enrollment committed but the exact
// protected ticket file could not be safely removed. Callers must not repeat
// enrollment; Result identifies the committed enrollment.
type EnrollmentFileCleanupError struct {
	Result          EnrollmentResult
	TicketFileState EnrollmentTicketFileState
	cause           error
}

func (e *EnrollmentFileCleanupError) Error() string {
	return "Application enrollment committed but ticket file cleanup failed"
}

func (e *EnrollmentFileCleanupError) Unwrap() error {
	if e == nil {
		return nil
	}
	return e.cause
}

// EnrollApplicationFromFile consumes the protected ticket file written by the
// Operator CLI. It retains the file when enrollment fails and removes it only
// after a validated enrollment response.
func EnrollApplicationFromFile(ctx context.Context, config EnrollmentFileConfig) (EnrollmentResult, error) {
	if runtime.GOOS == "windows" {
		return EnrollmentResult{}, errors.New("Application enrollment from a protected file requires the Application Unix listener")
	}
	return enrollApplicationFromFile(ctx, config, EnrollApplication, (*applicationEnrollmentTicketFile).remove)
}

func enrollApplicationFromFile(
	ctx context.Context,
	config EnrollmentFileConfig,
	enroll func(context.Context, EnrollmentConfig) (EnrollmentResult, error),
	remove func(*applicationEnrollmentTicketFile) (EnrollmentTicketFileState, error),
) (EnrollmentResult, error) {
	ticketFile, err := openApplicationEnrollmentTicket(config.TicketPath)
	if err != nil {
		return EnrollmentResult{}, err
	}
	defer ticketFile.close()

	enrollmentConfig := EnrollmentConfig{
		SocketPath:    config.SocketPath,
		NodePrincipal: config.NodePrincipal,
		Ticket:        ticketFile.ticket,
		Signer:        config.Signer,
		HTTPClient:    config.HTTPClient,
	}
	defer clear(enrollmentConfig.Ticket.value[:])
	result, err := enroll(ctx, enrollmentConfig)
	if err != nil {
		return EnrollmentResult{}, err
	}
	state, err := remove(ticketFile)
	if err != nil {
		return result, &EnrollmentFileCleanupError{Result: result, TicketFileState: state, cause: err}
	}
	return result, nil
}

// EnrollApplication performs the one-use root possession flow over the
// protected Application Unix listener. Key custody remains with Signer.
func EnrollApplication(ctx context.Context, config EnrollmentConfig) (EnrollmentResult, error) {
	defer clear(config.Ticket.value[:])
	if config.Signer == nil || config.Ticket.value == [identitycontract.ApplicationEnrollmentTicketBytes]byte{} || !adapter.ValidPrincipalID(config.NodePrincipal) {
		return EnrollmentResult{}, fmt.Errorf("Application enrollment configuration is invalid")
	}
	httpClient, err := unixHTTPClient(strings.TrimSpace(config.SocketPath), config.HTTPClient)
	if err != nil {
		return EnrollmentResult{}, err
	}
	ticket := config.Ticket.value
	defer clear(ticket[:])
	result, err := adapter.NewEnrollmentClient(httpClient, "http://localhost", config.Signer, config.NodePrincipal, nil).Enroll(ctx, ticket)
	if err != nil {
		return EnrollmentResult{}, err
	}
	return EnrollmentResult{Principal: result.Principal, CredentialID: result.CredentialID, GrantID: result.GrantID, GrantExpiresAt: result.GrantExpiresAt}, nil
}

type applicationEnrollmentTicketFile struct {
	name   string
	root   *os.Root
	dir    *os.File
	file   *os.File
	info   os.FileInfo
	ticket ApplicationEnrollmentTicket
}

func openApplicationEnrollmentTicket(path string) (*applicationEnrollmentTicketFile, error) {
	if path == "" || path != strings.TrimSpace(path) || strings.ContainsRune(path, '\x00') || !filepath.IsAbs(path) {
		return nil, fmt.Errorf("Application enrollment ticket file is invalid")
	}
	linkInfo, err := os.Lstat(path)
	if err != nil || !linkInfo.Mode().IsRegular() || runtime.GOOS != "windows" && linkInfo.Mode().Perm()&0o077 != 0 {
		return nil, fmt.Errorf("Application enrollment ticket file is invalid")
	}
	parentInfo, err := os.Lstat(filepath.Dir(path))
	if err != nil || !parentInfo.IsDir() || parentInfo.Mode()&os.ModeSymlink != 0 ||
		runtime.GOOS != "windows" && parentInfo.Mode().Perm()&0o077 != 0 {
		return nil, fmt.Errorf("Application enrollment ticket file is invalid")
	}
	root, err := os.OpenRoot(filepath.Dir(path))
	if err != nil {
		return nil, fmt.Errorf("Application enrollment ticket file is unavailable")
	}
	rootInfo, err := root.Stat(".")
	if err != nil || !os.SameFile(parentInfo, rootInfo) {
		_ = root.Close()
		return nil, fmt.Errorf("Application enrollment ticket file is invalid")
	}
	name := filepath.Base(path)
	rootLinkInfo, err := root.Lstat(name)
	if err != nil || !rootLinkInfo.Mode().IsRegular() || !os.SameFile(linkInfo, rootLinkInfo) {
		_ = root.Close()
		return nil, fmt.Errorf("Application enrollment ticket file is invalid")
	}
	directory, err := root.Open(".")
	if err != nil {
		_ = root.Close()
		return nil, fmt.Errorf("Application enrollment ticket file is unavailable")
	}
	file, err := root.Open(name)
	if err != nil {
		_ = directory.Close()
		_ = root.Close()
		return nil, fmt.Errorf("Application enrollment ticket file is unavailable")
	}
	info, err := file.Stat()
	if err != nil || !info.Mode().IsRegular() || !os.SameFile(rootLinkInfo, info) {
		_ = file.Close()
		_ = directory.Close()
		_ = root.Close()
		return nil, fmt.Errorf("Application enrollment ticket file is invalid")
	}
	const encodedTicketBytes = (identitycontract.ApplicationEnrollmentTicketBytes*8 + 5) / 6
	raw, err := io.ReadAll(io.LimitReader(file, encodedTicketBytes+1))
	if err != nil {
		_ = file.Close()
		_ = directory.Close()
		_ = root.Close()
		return nil, fmt.Errorf("Application enrollment ticket file is unavailable")
	}
	defer clear(raw)
	if len(raw) != base64.RawURLEncoding.EncodedLen(identitycontract.ApplicationEnrollmentTicketBytes) {
		_ = file.Close()
		_ = directory.Close()
		_ = root.Close()
		return nil, fmt.Errorf("Application enrollment ticket file is invalid")
	}
	ticket, ok := decodeApplicationEnrollmentTicketBytes(raw)
	if !ok {
		_ = file.Close()
		_ = directory.Close()
		_ = root.Close()
		return nil, fmt.Errorf("Application enrollment ticket file is invalid")
	}
	return &applicationEnrollmentTicketFile{name: name, root: root, dir: directory, file: file, info: info, ticket: ticket}, nil
}

func decodeApplicationEnrollmentTicketBytes(encoded []byte) (ApplicationEnrollmentTicket, bool) {
	var result ApplicationEnrollmentTicket
	if len(encoded) != base64.RawURLEncoding.EncodedLen(len(result.value)) {
		return result, false
	}
	decoded := result.value[:]
	count, err := base64.RawURLEncoding.Decode(decoded, encoded)
	if err != nil || count != len(decoded) || result.value == [identitycontract.ApplicationEnrollmentTicketBytes]byte{} {
		clear(decoded)
		return ApplicationEnrollmentTicket{}, false
	}
	canonical := make([]byte, len(encoded))
	base64.RawURLEncoding.Encode(canonical, decoded)
	matches := bytes.Equal(canonical, encoded)
	clear(canonical)
	if !matches {
		clear(decoded)
		return ApplicationEnrollmentTicket{}, false
	}
	return result, true
}

func (f *applicationEnrollmentTicketFile) close() {
	if f == nil {
		return
	}
	clear(f.ticket.value[:])
	if f.file != nil {
		_ = f.file.Close()
		f.file = nil
	}
	if f.dir != nil {
		_ = f.dir.Close()
		f.dir = nil
	}
	if f.root != nil {
		_ = f.root.Close()
		f.root = nil
	}
}

func (f *applicationEnrollmentTicketFile) remove() (EnrollmentTicketFileState, error) {
	if f == nil || f.file == nil {
		return EnrollmentTicketFileUnknown, errors.New("Application enrollment ticket cleanup failed")
	}
	if err := f.file.Close(); err != nil {
		f.file = nil
		return f.currentTicketFileState(), errors.New("Application enrollment ticket cleanup failed")
	}
	f.file = nil
	current, err := f.root.Lstat(f.name)
	if err != nil || !current.Mode().IsRegular() || !os.SameFile(f.info, current) {
		return EnrollmentTicketFileUnknown, errors.New("Application enrollment ticket cleanup failed")
	}
	if runtime.GOOS == "windows" {
		if err := f.root.Remove(f.name); err != nil {
			return f.currentTicketFileState(), errors.New("Application enrollment ticket cleanup failed")
		}
		return EnrollmentTicketFileRetired, nil
	}
	return f.retireUnixTicket()
}

func (f *applicationEnrollmentTicketFile) retireUnixTicket() (EnrollmentTicketFileState, error) {
	quarantineName, quarantine, err := createTicketQuarantine(f.root)
	if err != nil {
		return f.currentTicketFileState(), errors.New("Application enrollment ticket cleanup failed")
	}
	if closeErr := quarantine.Close(); closeErr != nil {
		_ = f.root.Remove(quarantineName)
		return f.currentTicketFileState(), errors.New("Application enrollment ticket cleanup failed")
	}
	if err := f.root.Rename(f.name, quarantineName); err != nil {
		_ = f.root.Remove(quarantineName)
		return f.currentTicketFileState(), errors.New("Application enrollment ticket cleanup failed")
	}
	moved, err := f.root.Lstat(quarantineName)
	if err != nil || !moved.Mode().IsRegular() || !os.SameFile(f.info, moved) {
		if restoreErr := restoreCapturedTicket(f.root, quarantineName, f.name); restoreErr != nil {
			return EnrollmentTicketFileUnknown, errors.New("Application enrollment ticket cleanup failed and file state is unknown")
		}
		return EnrollmentTicketFileUnknown, errors.New("Application enrollment ticket cleanup failed")
	}
	if err := f.root.Remove(quarantineName); err != nil {
		if restoreErr := restoreCapturedTicket(f.root, quarantineName, f.name); restoreErr != nil {
			return EnrollmentTicketFileUnknown, errors.New("Application enrollment ticket cleanup failed and file state is unknown")
		}
		return f.currentTicketFileState(), errors.New("Application enrollment ticket cleanup failed")
	}
	if err := f.dir.Sync(); err != nil {
		return EnrollmentTicketFileRetired, errors.New("Application enrollment ticket cleanup durability is unknown")
	}
	return EnrollmentTicketFileRetired, nil
}

func (f *applicationEnrollmentTicketFile) currentTicketFileState() EnrollmentTicketFileState {
	if f == nil || f.root == nil {
		return EnrollmentTicketFileUnknown
	}
	info, err := f.root.Lstat(f.name)
	if err == nil && info.Mode().IsRegular() && os.SameFile(f.info, info) {
		return EnrollmentTicketFileRetained
	}
	return EnrollmentTicketFileUnknown
}

func createTicketQuarantine(root *os.Root) (string, *os.File, error) {
	for range 16 {
		var random [16]byte
		if _, err := rand.Read(random[:]); err != nil {
			return "", nil, err
		}
		name := ".ardents-consumed-ticket-" + hex.EncodeToString(random[:])
		file, err := root.OpenFile(name, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
		if err == nil {
			return name, file, nil
		}
		if !errors.Is(err, os.ErrExist) {
			return "", nil, err
		}
	}
	return "", nil, errors.New("Application enrollment ticket cleanup failed")
}

func restoreCapturedTicket(root *os.Root, capturedName, ticketName string) error {
	if err := root.Link(capturedName, ticketName); err != nil {
		return err
	}
	if err := root.Remove(capturedName); err != nil {
		return err
	}
	return nil
}
