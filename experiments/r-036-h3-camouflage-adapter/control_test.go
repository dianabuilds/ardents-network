//go:build ignore

package main

import (
	"bufio"
	"net/http"
	"strings"
	"testing"
)

func TestClientReadiness(t *testing.T) {
	input := "VERSION 1\nSTATUS TYPE=version\nCMETHOD webtunnel socks5 127.0.0.1:4123\nCMETHODS DONE\n"
	got, _, err := readReadiness(strings.NewReader(input), "webtunnel", "CMETHOD")
	if err != nil {
		t.Fatal(err)
	}
	if got.address != "127.0.0.1:4123" {
		t.Fatalf("address = %q", got.address)
	}
}

func TestWorkloadIsExactHTTPRequest(t *testing.T) {
	seed := strings.Repeat("01", 32)
	work, err := makeWorkload(seed)
	if err != nil {
		t.Fatal(err)
	}
	if len(work.request) != 512 || len(work.response) != 64<<10 {
		t.Fatalf("request/response sizes = %d/%d", len(work.request), len(work.response))
	}
	request, err := http.ReadRequest(bufio.NewReader(strings.NewReader(string(work.request))))
	if err != nil {
		t.Fatal(err)
	}
	if request.Header.Get("X-Ardents-Nonce") == "" || request.Host != "bridge.invalid" {
		t.Fatal("request omitted nonce or fixed host")
	}
}

func TestServerReadinessArgs(t *testing.T) {
	input := "VERSION 1\nSMETHOD obfs4 127.0.0.1:4123 ARGS:cert=abc,iat-mode=0\nSMETHODS DONE\n"
	got, _, err := readReadiness(strings.NewReader(input), "obfs4", "SMETHOD")
	if err != nil {
		t.Fatal(err)
	}
	if got.args["cert"] != "abc" || got.args["iat-mode"] != "0" {
		t.Fatalf("args = %#v", got.args)
	}
}

func TestMalformedControlRejected(t *testing.T) {
	tests := map[string]string{
		"missing version":   "CMETHOD webtunnel socks5 127.0.0.1:1\nCMETHODS DONE\n",
		"duplicate method":  "VERSION 1\nCMETHOD webtunnel socks5 127.0.0.1:1\nCMETHOD webtunnel socks5 127.0.0.1:2\n",
		"wrong method":      "VERSION 1\nCMETHOD obfs4 socks5 127.0.0.1:1\nCMETHODS DONE\n",
		"wrong socks":       "VERSION 1\nCMETHOD webtunnel socks4 127.0.0.1:1\nCMETHODS DONE\n",
		"non loopback":      "VERSION 1\nCMETHOD webtunnel socks5 192.0.2.3:1\nCMETHODS DONE\n",
		"terminal":          "VERSION 1\nCMETHOD-ERROR webtunnel failed\n",
		"early done":        "VERSION 1\nCMETHODS DONE\n",
		"duplicate version": "VERSION 1\nVERSION 1\n",
		"non ASCII keyword": "VERSION 1\nM\u00c9THOD value\n",
	}
	for name, input := range tests {
		t.Run(name, func(t *testing.T) {
			if _, _, err := readReadiness(strings.NewReader(input), "webtunnel", "CMETHOD"); err == nil {
				t.Fatal("malformed transcript accepted")
			}
		})
	}
}

func TestControlLimits(t *testing.T) {
	long := "VERSION 1\n" + strings.Repeat("X", maxControlLine+1) + "\n"
	if _, _, err := readReadiness(strings.NewReader(long), "webtunnel", "CMETHOD"); err == nil {
		t.Fatal("oversized line accepted")
	}
	many := "VERSION 1\n" + strings.Repeat("STATUS X\n", maxControlLines)
	if _, _, err := readReadiness(strings.NewReader(many), "webtunnel", "CMETHOD"); err == nil {
		t.Fatal("oversized transcript accepted")
	}
}
