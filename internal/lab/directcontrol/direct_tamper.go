package directcontrol

import (
	"context"
	"encoding/binary"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"os"
	"path/filepath"
	"runtime"
	"sync"
	"sync/atomic"
	"time"
)

const directTamperSchema = "carrier-lab-direct-tamper/v1"

type directTamperConfig struct {
	SchemaVersion  string `json:"schema_version"`
	RunID          string `json:"run_id"`
	ListenAddress  string `json:"listen_address"`
	ServiceAddress string `json:"service_address"`
}

type directTamperResult struct {
	SchemaVersion           string `json:"schema_version"`
	RunID                   string `json:"run_id"`
	Status                  string `json:"status"`
	TerminalResult          string `json:"terminal_result"`
	ProtectedRecordModified bool   `json:"protected_record_modified"`
	ApplicationBytesSeen    bool   `json:"application_bytes_seen"`
	ElapsedMilliseconds     int64  `json:"elapsed_milliseconds"`
	HeapAllocBytes          uint64 `json:"heap_alloc_bytes"`
	Goroutines              int    `json:"goroutines"`
	Failure                 string `json:"failure,omitempty"`
}

// RunTamper executes the fixed modified-record fault without receiving
// Target, Instance, canary, payload, naming, discovery, or topology knowledge.
func RunTamper(ctx context.Context, configPath, evidenceDir string) error {
	started := time.Now()
	config, err := readDirectTamperConfig(configPath)
	if err != nil {
		return err
	}
	if err := requireDirectDirectory(evidenceDir); err != nil {
		return err
	}
	result := directTamperResult{
		SchemaVersion: directTamperSchema, RunID: config.RunID, Status: "failed", TerminalResult: "explicit_failure",
	}
	modified, runErr := runDirectTamper(ctx, config, evidenceDir)
	result.ProtectedRecordModified = modified
	if runErr == nil && modified {
		result.Status = "passed"
		result.TerminalResult = "fault_injected"
	} else if runErr != nil {
		result.Failure = runErr.Error()
	}
	var memory runtime.MemStats
	runtime.ReadMemStats(&memory)
	result.ElapsedMilliseconds = time.Since(started).Milliseconds()
	result.HeapAllocBytes = memory.HeapAlloc
	result.Goroutines = runtime.NumGoroutine()
	evidenceErr := writeDirectJSON(filepath.Join(evidenceDir, "result.json"), result)
	return errors.Join(runErr, evidenceErr)
}

func readDirectTamperConfig(path string) (directTamperConfig, error) {
	file, err := os.Open(path)
	if err != nil {
		return directTamperConfig{}, err
	}
	defer file.Close()
	decoder := json.NewDecoder(file)
	decoder.DisallowUnknownFields()
	var config directTamperConfig
	if err := decoder.Decode(&config); err != nil {
		return directTamperConfig{}, err
	}
	if config.SchemaVersion != directTamperSchema || !runIDPattern.MatchString(config.RunID) || config.ListenAddress == "" || config.ServiceAddress == "" {
		return directTamperConfig{}, errors.New("invalid Direct TLS tamper configuration")
	}
	return config, nil
}

func runDirectTamper(ctx context.Context, config directTamperConfig, evidenceDir string) (bool, error) {
	listener, err := (&net.ListenConfig{}).Listen(ctx, "tcp", config.ListenAddress)
	if err != nil {
		return false, err
	}
	defer listener.Close()
	stopClose := context.AfterFunc(ctx, func() { _ = listener.Close() })
	defer stopClose()
	if err := writeDirectJSON(filepath.Join(evidenceDir, "ready.json"), map[string]string{
		"schema_version": directTamperSchema, "run_id": config.RunID, "status": "ready",
	}); err != nil {
		return false, err
	}
	userConnection, err := acceptDirectConnection(listener, directDeadline)
	if err != nil {
		return false, err
	}
	defer userConnection.Close()
	serviceConnection, err := (&net.Dialer{Timeout: directDeadline}).DialContext(ctx, "tcp", config.ServiceAddress)
	if err != nil {
		return false, err
	}
	defer serviceConnection.Close()
	deadline := time.Now().Add(directDeadline)
	_ = userConnection.SetDeadline(deadline)
	_ = serviceConnection.SetDeadline(deadline)

	applicationSent := make(chan struct{})
	var armOnce sync.Once
	var modified atomic.Bool
	errorsSeen := make(chan error, 2)
	go func() {
		encryptedRecords := 0
		errorsSeen <- copyDirectTLSRecords(userConnection, serviceConnection, func(recordType byte, payload []byte) {
			if recordType == 23 {
				encryptedRecords++
				if encryptedRecords == 2 {
					armOnce.Do(func() { close(applicationSent) })
				}
			}
		})
	}()
	go func() {
		errorsSeen <- copyDirectTLSRecords(serviceConnection, userConnection, func(recordType byte, payload []byte) {
			if recordType != 23 || len(payload) == 0 || modified.Load() {
				return
			}
			select {
			case <-applicationSent:
				payload[len(payload)-1] ^= 0x01
				modified.Store(true)
			default:
			}
		})
	}()
	firstErr := <-errorsSeen
	_ = userConnection.Close()
	_ = serviceConnection.Close()
	secondErr := <-errorsSeen
	if !modified.Load() {
		return false, errors.Join(errors.New("protected TLS record was not modified"), normalizeProxyError(firstErr), normalizeProxyError(secondErr))
	}
	return true, nil
}

func copyDirectTLSRecords(reader io.Reader, writer io.Writer, inspect func(recordType byte, payload []byte)) error {
	for {
		header := make([]byte, 5)
		if _, err := io.ReadFull(reader, header); err != nil {
			return err
		}
		size := int(binary.BigEndian.Uint16(header[3:5]))
		if size < 1 || size > 18*1024 {
			return fmt.Errorf("TLS record length %d is out of bounds", size)
		}
		payload := make([]byte, size)
		if _, err := io.ReadFull(reader, payload); err != nil {
			return err
		}
		inspect(header[0], payload)
		if err := writeDirectBytes(writer, header); err != nil {
			return err
		}
		if err := writeDirectBytes(writer, payload); err != nil {
			return err
		}
	}
}

func normalizeProxyError(err error) error {
	if err == nil || errors.Is(err, io.EOF) || errors.Is(err, net.ErrClosed) {
		return nil
	}
	return err
}
