package siteexperiment

import (
	"bufio"
	"bytes"
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"net/http"
)

const (
	httpRequestSize  = 512
	httpResponseSize = 64 * 1024
)

type httpWorkloadResult struct {
	StatusCode    int
	ResponseBytes int
	EOF           bool
}

func buildHTTPRequest(nonce []byte) ([]byte, error) {
	if len(nonce) != 32 {
		return nil, errors.New("HTTP workload nonce must be 32 bytes")
	}
	prefix := fmt.Sprintf("GET /%s HTTP/1.1\r\nHost: site.reference\r\nConnection: close\r\nCache-Control: no-store\r\nX-GateC-Padding: ", hex.EncodeToString(nonce))
	suffix := "\r\n\r\n"
	paddingLength := httpRequestSize - len(prefix) - len(suffix)
	if paddingLength < 1 {
		return nil, errors.New("HTTP workload headers exceed the fixed request size")
	}
	return []byte(prefix + string(bytes.Repeat([]byte{'a'}, paddingLength)) + suffix), nil
}

func serveHTTPApplication(stream io.ReadWriteCloser, nonce []byte) error {
	if stream == nil {
		return errors.New("HTTP Application stream is required")
	}
	defer stream.Close()
	expected, err := buildHTTPRequest(nonce)
	if err != nil {
		return err
	}
	request := make([]byte, httpRequestSize)
	if _, err := io.ReadFull(stream, request); err != nil {
		return err
	}
	if !bytes.Equal(request, expected) {
		return errors.New("HTTP Application received a non-canonical request")
	}
	header := []byte("HTTP/1.1 200 OK\r\nContent-Length: 65536\r\nContent-Type: application/octet-stream\r\nConnection: close\r\nCache-Control: no-store\r\n\r\n")
	if _, err := stream.Write(header); err != nil {
		return err
	}
	_, err = stream.Write(deterministicResponse(nonce))
	return err
}

func executeHTTPWorkload(stream io.ReadWriteCloser, nonce []byte) (httpWorkloadResult, error) {
	if stream == nil {
		return httpWorkloadResult{}, errors.New("HTTP client stream is required")
	}
	request, err := buildHTTPRequest(nonce)
	if err != nil {
		return httpWorkloadResult{}, err
	}
	if _, err := stream.Write(request); err != nil {
		return httpWorkloadResult{}, err
	}
	reader := bufio.NewReader(stream)
	response, err := http.ReadResponse(reader, nil)
	if err != nil {
		return httpWorkloadResult{}, err
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK || response.ContentLength != httpResponseSize ||
		response.Header.Get("Content-Type") != "application/octet-stream" || response.Header.Get("Cache-Control") != "no-store" ||
		!response.Close || response.Header.Get("Location") != "" {
		return httpWorkloadResult{}, errors.New("HTTP response status or headers violate the Gate C contract")
	}
	body, err := io.ReadAll(io.LimitReader(response.Body, httpResponseSize+1))
	if err != nil {
		return httpWorkloadResult{}, err
	}
	if len(body) != httpResponseSize || !bytes.Equal(body, deterministicResponse(nonce)) {
		return httpWorkloadResult{}, errors.New("HTTP response body is incomplete or invalid")
	}
	if _, err := reader.ReadByte(); !errors.Is(err, io.EOF) {
		return httpWorkloadResult{}, errors.New("HTTP response did not terminate at the expected EOF")
	}
	return httpWorkloadResult{StatusCode: response.StatusCode, ResponseBytes: len(body), EOF: true}, nil
}

func deterministicResponse(nonce []byte) []byte {
	result := make([]byte, 0, httpResponseSize)
	var counter [8]byte
	for block := uint64(0); len(result) < httpResponseSize; block++ {
		binary.BigEndian.PutUint64(counter[:], block)
		hash := sha256.New()
		_, _ = hash.Write(nonce)
		_, _ = hash.Write(counter[:])
		result = hash.Sum(result)
	}
	return result[:httpResponseSize]
}
