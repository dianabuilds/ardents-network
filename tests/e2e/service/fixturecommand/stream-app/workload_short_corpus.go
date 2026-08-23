package main

import (
	"encoding/hex"
	"errors"
	"fmt"
	"io"
)

const shortCorpus = "h3-s5-http-512-65536-v1"

// ExchangeShort carries the frozen Stage 5 request/response corpus through the
// same opaque Application framing as Exchange.
func ExchangeShort(connection io.ReadWriter, role string, sendSeed, expectSeed [32]byte,
	write func([]byte) (int, error), progress func(uint32),
) (Observation, error) {
	if role != "client" && role != "publisher" {
		return Observation{}, errors.New("short corpus role is invalid")
	}
	requestSeed := sendSeed
	if role == "publisher" {
		requestSeed = expectSeed
	}
	request, err := newShortRequest(requestSeed)
	if err != nil {
		return Observation{}, err
	}
	sendSource, expectSource := byteFiller(&generator{seed: sendSeed}), byteFiller(&generator{seed: expectSeed})
	sendCount, receiveCount := 512, 64<<10
	if role == "client" {
		sendSource = request
	} else {
		expectSource, sendCount, receiveCount = request, 64<<10, 512
	}
	result, err := exchange(connection, role, sendSeed, expectSeed, sendCount, receiveCount,
		write, progress, sendSource, expectSource)
	result.Corpus, result.RequestNonce = shortCorpus, requestSeed
	return result, err
}

type shortRequest struct {
	prefix []byte
	offset int
	body   generator
}

func newShortRequest(nonce [32]byte) (*shortRequest, error) {
	header := fmt.Sprintf("POST / HTTP/1.1\r\nHost: service.target\r\nContent-Type: application/octet-stream\r\n"+
		"X-Connection-Nonce: %s\r\nContent-Length: 324\r\n\r\n", hex.EncodeToString(nonce[:]))
	if len(header) != 188 {
		return nil, errors.New("short HTTP request header is not canonical")
	}
	return &shortRequest{prefix: []byte(header), body: generator{seed: nonce}}, nil
}

func (source *shortRequest) fill(value []byte) {
	if source.offset < len(source.prefix) {
		count := copy(value, source.prefix[source.offset:])
		source.offset += count
		value = value[count:]
	}
	source.body.fill(value)
}
