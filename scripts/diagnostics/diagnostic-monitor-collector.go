//go:build ignore

package main

import (
	"crypto/sha256"
	"crypto/tls"
	"crypto/x509"
	"encoding/hex"
	"errors"
	"io"
	"log"
	"net"
	"net/http"
	"os"
	"strconv"
	"syscall"
	"time"
)

// openCollectorMetrics serves only fixed metrics to one pinned diagnostic client.
// It grants no panel, raw evidence, profile capture or product authority access.
func openCollectorMetrics(address string, container bool, delivery *monitorDelivery, directory, clientPin string) (*monitorView, error) {
	decoded, err := hex.DecodeString(clientPin)
	if err != nil || len(decoded) != sha256.Size {
		return nil, errors.New("select one SHA256 diagnostic client SPKI pin")
	}
	var pin [sha256.Size]byte
	copy(pin[:], decoded)
	files, err := collectorCertificateFiles(directory)
	if err != nil {
		return nil, err
	}
	certificate, err := tls.X509KeyPair(files[0], files[1])
	if err != nil {
		return nil, err
	}
	authorities := x509.NewCertPool()
	if !authorities.AppendCertsFromPEM(files[2]) {
		return nil, errors.New("invalid diagnostic client CA")
	}
	host, port, err := net.SplitHostPort(address)
	if err != nil {
		return nil, err
	}
	ip := net.ParseIP(host)
	numericPort, err := strconv.ParseUint(port, 10, 16)
	if err != nil || ip == nil || !ip.IsLoopback() && !(container && host == "0.0.0.0") || numericPort > 65535 {
		return nil, errors.New("collector metrics requires loopback or explicit container binding")
	}
	configuration := &tls.Config{
		MinVersion: tls.VersionTLS13, Certificates: []tls.Certificate{certificate},
		ClientAuth: tls.RequireAndVerifyClientCert, ClientCAs: authorities,
		VerifyConnection: func(state tls.ConnectionState) error {
			if len(state.PeerCertificates) == 0 || len(state.VerifiedChains) == 0 ||
				sha256.Sum256(state.PeerCertificates[0].RawSubjectPublicKeyInfo) != pin {
				return errors.New("diagnostic client unavailable")
			}
			return nil
		},
	}
	listener, err := net.Listen("tcp", address)
	if err != nil {
		return nil, err
	}
	bounded := &monitorListener{Listener: listener, slots: make(chan struct{}, 4)}
	view := &monitorView{listener: bounded, done: make(chan struct{})}
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", "no-store")
		w.Header().Set("X-Content-Type-Options", "nosniff")
		// Recheck certificate time validity on requests over retained TLS
		// connections; no live CA/pin reload or revocation service is implied.
		valid := false
		if r.TLS != nil && len(r.TLS.PeerCertificates) > 0 &&
			sha256.Sum256(r.TLS.PeerCertificates[0].RawSubjectPublicKeyInfo) == pin {
			now := time.Now()
			for _, chain := range r.TLS.VerifiedChains {
				current := true
				for _, cert := range chain {
					if now.Before(cert.NotBefore) || now.After(cert.NotAfter) {
						current = false
					}
				}
				valid = valid || current
			}
		}
		if !valid {
			http.Error(w, "diagnostic client unavailable", http.StatusForbidden)
			return
		}
		if r.Method != http.MethodGet || r.URL.Path != "/metrics" {
			http.NotFound(w, r)
			return
		}
		body, err := monitorMetrics(delivery.snapshot(), time.Now().UTC())
		if err != nil {
			http.Error(w, "monitor metrics unavailable", http.StatusServiceUnavailable)
			return
		}
		w.Header().Set("Content-Type", "text/plain; version=0.0.4; charset=utf-8")
		w.Write(body)
	})
	view.server = &http.Server{
		Handler: handler, TLSConfig: configuration, ErrorLog: log.New(io.Discard, "", 0),
		ReadHeaderTimeout: 3 * time.Second, ReadTimeout: 5 * time.Second, WriteTimeout: 5 * time.Second,
		IdleTimeout: 10 * time.Second, MaxHeaderBytes: 8192,
	}
	go func() {
		outcome := view.server.ServeTLS(bounded, "", "")
		if errors.Is(outcome, http.ErrServerClosed) {
			outcome = nil
		}
		view.outcome = outcome
		if outcome != nil {
			delivery.mu.Lock()
			delivery.state.MetricsFailed = true
			delivery.mu.Unlock()
		}
		close(view.done)
	}()
	return view, nil
}

// collectorCertificateFiles loads a fixed bounded tuple before listener effects.
// File/root close errors remain errors; no key or path is returned over HTTP.
func collectorCertificateFiles(directory string) (files [3][]byte, outcome error) {
	root, err := privateEvidenceRoot(directory)
	if err != nil {
		return files, err
	}
	defer func() { outcome = errors.Join(outcome, root.Close()) }()
	read := func(name string) (body []byte, outcome error) {
		file, err := root.OpenFile(name, os.O_RDONLY|syscall.O_NOFOLLOW|syscall.O_NONBLOCK, 0)
		if err != nil {
			return nil, err
		}
		defer func() { outcome = errors.Join(outcome, file.Close()) }()
		before, err := file.Stat()
		if err != nil {
			return nil, err
		}
		owner, ok := before.Sys().(*syscall.Stat_t)
		if !before.Mode().IsRegular() || before.Mode().Perm()&0077 != 0 || !ok ||
			owner.Uid != uint32(os.Geteuid()) || owner.Nlink != 1 || before.Size() < 0 || before.Size() > 64<<10 {
			return nil, errors.New("select bounded owned private diagnostic certificate files")
		}
		body, err = io.ReadAll(io.LimitReader(file, (64<<10)+1))
		if err != nil {
			return nil, err
		}
		after, err := file.Stat()
		if err != nil {
			return nil, err
		}
		named, err := root.Lstat(name)
		if err != nil {
			return nil, err
		}
		afterOwner, ok := after.Sys().(*syscall.Stat_t)
		if len(body) > 64<<10 || int64(len(body)) != before.Size() || after.Size() != before.Size() ||
			!after.ModTime().Equal(before.ModTime()) || !os.SameFile(before, named) ||
			after.Mode().Perm()&0077 != 0 || !ok || afterOwner.Uid != uint32(os.Geteuid()) || afterOwner.Nlink != 1 {
			return nil, errors.New("diagnostic certificate file changed during loading")
		}
		return body, nil
	}
	for index, name := range []string{"server.crt", "server.key", "client-ca.crt"} {
		files[index], outcome = read(name)
		if outcome != nil {
			return files, outcome
		}
	}
	return files, nil
}
