package camouflage

import (
	"strings"
	"testing"
)

func TestClientControlAcceptsOneCanonicalReadiness(t *testing.T) {
	t.Parallel()
	input := "VERSION 1\nSTATUS TYPE=version\nCMETHOD webtunnel socks5 127.0.0.1:4123\nCMETHODS DONE\n"
	address, transcript, err := readClientReadiness(strings.NewReader(input))
	if err != nil || address != "127.0.0.1:4123" || string(transcript) != input {
		t.Fatalf("client readiness = %q %q, %v", address, transcript, err)
	}
}

func TestControlRejectsMalformedOrOverBoundTranscripts(t *testing.T) {
	t.Parallel()
	clientCases := []string{
		"CMETHOD webtunnel socks5 127.0.0.1:1\nCMETHODS DONE\n",
		"VERSION 1\nCMETHOD webtunnel socks5 192.0.2.3:1\nCMETHODS DONE\n",
		"VERSION 1\nCMETHOD webtunnel socks4 127.0.0.1:1\nCMETHODS DONE\n",
		"VERSION 1\nCMETHOD webtunnel socks5 127.0.0.1:not-a-port\nCMETHODS DONE\n",
		"VERSION 1\nCMETHOD webtunnel socks5 127.0.0.1:http\nCMETHODS DONE\n",
		"VERSION 1\nCMETHOD webtunnel socks5 127.0.0.1:080\nCMETHODS DONE\n",
		"VERSION 1\nSTATUS good\x00bad\nCMETHOD webtunnel socks5 127.0.0.1:1\nCMETHODS DONE\n",
		"VERSION 1\nCMETHOD-ERROR webtunnel failed\n",
		"VERSION 1\n" + strings.Repeat("X", maximumControlLine+1) + "\n",
		"VERSION 1\n" + strings.Repeat("STATUS X\n", maximumControlLines),
	}
	for index, input := range clientCases {
		if _, _, err := readClientReadiness(strings.NewReader(input)); err == nil {
			t.Fatalf("client case %d accepted malformed control", index)
		}
	}
	serverCases := []string{
		"VERSION 1\nSMETHODS DONE\n",
		"VERSION 1\nSMETHOD obfs4 127.0.0.1:1\nSMETHODS DONE\n",
		"VERSION 1\nSMETHOD-ERROR webtunnel failed\n",
	}
	for index, input := range serverCases {
		if _, _, err := readServerReadiness(strings.NewReader(input)); err == nil {
			t.Fatalf("server case %d accepted malformed control", index)
		}
	}
}

func TestBoundedCandidateOutputQuarantinesOverflow(t *testing.T) {
	t.Parallel()
	output := boundedOutput{limit: 4}
	if written, err := output.Write([]byte("secret")); err != nil || written != 6 {
		t.Fatalf("bounded write = %d, %v", written, err)
	}
	if string(output.bytes()) != "secr" || !output.tooLarge() {
		t.Fatalf("bounded output = %q exceeded=%t", output.bytes(), output.tooLarge())
	}
}
