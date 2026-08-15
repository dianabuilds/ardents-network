//go:build ignore

package main

import (
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"errors"
)

type workload struct {
	clientCanary []byte
	request      []byte
	serverCanary []byte
	response     []byte
}

func makeWorkload(seedText string) (workload, error) {
	seed, err := hex.DecodeString(seedText)
	if err != nil || len(seed) != 32 {
		return workload{}, errors.New("workload seed must be exactly 32 bytes of hexadecimal")
	}
	return workload{
		clientCanary: expand(seed, "client-canary", 32),
		request:      makeHTTPRequest(expand(seed, "request-nonce", 32), seed),
		serverCanary: expand(seed, "server-canary", 32),
		response:     expand(seed, "response", 64<<10),
	}, nil
}

func makeHTTPRequest(nonce, seed []byte) []byte {
	nonceText := hex.EncodeToString(nonce)
	prefix := "GET /r036/" + nonceText + " HTTP/1.1\r\n" +
		"Host: bridge.invalid\r\n" +
		"Accept: application/octet-stream\r\n" +
		"Connection: close\r\n" +
		"X-Ardents-Nonce: " + nonceText + "\r\n" +
		"Content-Length: 0\r\nX-Pad: "
	suffix := "\r\n\r\n"
	paddingLength := 512 - len(prefix) - len(suffix)
	padding := hex.EncodeToString(expand(seed, "request-padding", (paddingLength+1)/2))
	return []byte(prefix + padding[:paddingLength] + suffix)
}

func expand(seed []byte, label string, size int) []byte {
	result := make([]byte, 0, size)
	for counter := uint32(0); len(result) < size; counter++ {
		input := append(append([]byte{}, seed...), []byte(label)...)
		encoded := make([]byte, 4)
		binary.BigEndian.PutUint32(encoded, counter)
		digest := sha256.Sum256(append(input, encoded...))
		result = append(result, digest[:]...)
	}
	return result[:size]
}

func equalBytes(left, right []byte) bool {
	if len(left) != len(right) {
		return false
	}
	var difference byte
	for index := range left {
		difference |= left[index] ^ right[index]
	}
	return difference == 0
}
