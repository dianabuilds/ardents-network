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
	offer blockedAdmissionOffer
	err   error
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
		go runCapacityOffer(client, seed, index, start, cadence, outcomes)
	}
	result.Outcomes = make([]blockedAdmissionOffer, offers)
	for range offers {
		observed := <-outcomes
		if observed.err != nil {
			t.Fatalf("capacity offer %d: %v", observed.offer.Ordinal, observed.err)
		}
		if observed.offer.Refused {
			result.Refused++
		}
		result.MaximumMillis = max(result.MaximumMillis, observed.offer.RefusalMillis)
		result.Outcomes[observed.offer.Ordinal] = observed.offer
	}
	writeBlockedJSON(t, "/run/evidence/admission-result.json", result)
	if err := json.NewEncoder(os.Stdout).Encode(result); err != nil {
		t.Fatal(err)
	}
}

func runCapacityOffer(client *http.Client, seed []byte, index int, origin time.Time, cadence time.Duration,
	result chan<- capacityOfferOutcome,
) {
	at := origin.Add(time.Duration(index) * cadence)
	if wait := time.Until(at); wait > 0 {
		time.Sleep(wait)
	}
	started := time.Now()
	offer := blockedAdmissionOffer{Ordinal: uint16(index),
		ScheduledOffsetMillis: uint32((time.Duration(index) * cadence).Milliseconds()),
		StartedOffsetMillis:   uint32(max(int64(0), started.Sub(origin).Milliseconds()))}
	request, requestErr := http.NewRequest(http.MethodGet, "https://203.0.113.8:8480/entry", nil)
	if requestErr != nil {
		result <- capacityOfferOutcome{offer: offer, err: requestErr}
		return
	}
	request.Host = "front.example"
	ordinal := make([]byte, 8)
	binary.BigEndian.PutUint64(ordinal, uint64(index))
	canary := sha256.Sum256(append(append([]byte(nil), seed...), ordinal...))
	canaryDigest := sha256.Sum256(canary[:])
	offer.CanarySHA256 = hex.EncodeToString(canaryDigest[:])
	request.Header.Set("X-Ardents-Probe", hex.EncodeToString(canary[:]))
	response, responseErr := client.Do(request)
	requestErr = errors.Join(requestErr, responseErr)
	refused := false
	if response != nil {
		_, _ = io.Copy(io.Discard, io.LimitReader(response.Body, 4096))
		requestErr = errors.Join(requestErr, response.Body.Close())
		refused = response.StatusCode == http.StatusServiceUnavailable
	}
	offer.Refused = refused
	offer.RefusalMillis = uint32(time.Since(started).Milliseconds())
	result <- capacityOfferOutcome{offer: offer, err: requestErr}
}
