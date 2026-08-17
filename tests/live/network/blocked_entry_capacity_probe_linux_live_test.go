//go:build linux && live

package network_test

import (
	"crypto/sha256"
	"crypto/tls"
	"crypto/x509"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"os"
	"strconv"
	"testing"
	"time"
)

type capacityOfferOutcome struct {
	refused bool
	elapsed uint32
	err     error
}

func runBlockedCapacityProbe(t *testing.T) {
	t.Helper()
	offers, err := strconv.Atoi(os.Getenv("ARDENTS_CAPACITY_OFFERS"))
	if err != nil || offers < 1 || offers > 1_000 {
		t.Fatalf("invalid capacity offer count %q", os.Getenv("ARDENTS_CAPACITY_OFFERS"))
	}
	cadence, err := time.ParseDuration(os.Getenv("ARDENTS_CAPACITY_CADENCE"))
	if err != nil || cadence < 0 || cadence > 100*time.Millisecond {
		t.Fatalf("invalid capacity cadence %q", os.Getenv("ARDENTS_CAPACITY_CADENCE"))
	}
	roots := x509.NewCertPool()
	raw, err := os.ReadFile("/run/secure/front-cert.pem")
	if err != nil || !roots.AppendCertsFromPEM(raw) {
		t.Fatal("capacity probe front root is invalid")
	}
	transport := &http.Transport{DisableKeepAlives: true, TLSClientConfig: &tls.Config{
		MinVersion: tls.VersionTLS13, RootCAs: roots, ServerName: "front.example"}}
	client := &http.Client{Transport: transport, Timeout: time.Second}
	defer transport.CloseIdleConnections()
	result := blockedAdmissionResult{Schema: "ardents-h3-s5-admission-result-v1", Offers: uint16(offers)}
	seed, err := os.ReadFile("/run/secure/corpus-seed.bin")
	if err != nil || len(seed) != 32 {
		t.Fatal("capacity offer corpus seed is invalid")
	}
	outcomes := make(chan capacityOfferOutcome, offers)
	start := time.Now().Add(100 * time.Millisecond)
	for index := range offers {
		go runCapacityOffer(client, seed, index, start.Add(time.Duration(index)*cadence), outcomes)
	}
	for index := range offers {
		observed := <-outcomes
		if observed.err != nil {
			t.Fatalf("capacity offer %d: %v", index, observed.err)
		}
		if observed.refused {
			result.Refused++
		}
		result.MaximumMillis = max(result.MaximumMillis, observed.elapsed)
	}
	writeBlockedJSON(t, "/run/evidence/admission-result.json", result)
	if err := json.NewEncoder(os.Stdout).Encode(result); err != nil {
		t.Fatal(err)
	}
}

func runCapacityOffer(client *http.Client, seed []byte, index int, at time.Time,
	result chan<- capacityOfferOutcome,
) {
	if wait := time.Until(at); wait > 0 {
		time.Sleep(wait)
	}
	started := time.Now()
	request, requestErr := http.NewRequest(http.MethodGet, "https://203.0.113.8:8480/entry", nil)
	if requestErr != nil {
		result <- capacityOfferOutcome{err: requestErr}
		return
	}
	request.Host = "front.example"
	ordinal := make([]byte, 8)
	binary.BigEndian.PutUint64(ordinal, uint64(index))
	canary := sha256.Sum256(append(append([]byte(nil), seed...), ordinal...))
	request.Header.Set("X-Ardents-Probe", hex.EncodeToString(canary[:]))
	response, responseErr := client.Do(request)
	requestErr = errors.Join(requestErr, responseErr)
	refused := false
	if response != nil {
		_, _ = io.Copy(io.Discard, io.LimitReader(response.Body, 4096))
		requestErr = errors.Join(requestErr, response.Body.Close())
		refused = response.StatusCode == http.StatusServiceUnavailable
	}
	result <- capacityOfferOutcome{refused, uint32(time.Since(started).Milliseconds()), requestErr}
}
