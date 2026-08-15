//go:build ignore

package main

import (
	"bufio"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/base64"
	"encoding/pem"
	"errors"
	"io"
	"math/big"
	"net"
	"net/http"
	"sync"
	"sync/atomic"
	"time"
)

const webTunnelPath = "/ardents-r036"

type fixture struct {
	listener net.Listener
	done     chan error
}

func startEcho(work workload) (*fixture, string, error) {
	listener, err := net.Listen("tcp4", "127.0.0.1:0")
	if err != nil {
		return nil, "", err
	}
	f := &fixture{listener: listener, done: make(chan error, 1)}
	go func() {
		conn, err := listener.Accept()
		if err != nil {
			f.done <- err
			return
		}
		defer conn.Close()
		request := make([]byte, len(work.clientCanary)+len(work.request))
		if _, err = io.ReadFull(conn, request); err != nil {
			f.done <- err
			return
		}
		expected := append(append([]byte{}, work.clientCanary...), work.request...)
		if !equalBytes(request, expected) {
			f.done <- errors.New("echo fixture received corrupted request")
			return
		}
		payload := append(append([]byte{}, work.serverCanary...), work.response...)
		_, err = conn.Write(payload)
		f.done <- err
	}()
	return f, listener.Addr().String(), nil
}

func (f *fixture) close() error {
	_ = f.listener.Close()
	select {
	case err := <-f.done:
		if errors.Is(err, net.ErrClosed) {
			return nil
		}
		return err
	case <-time.After(time.Second):
		return errors.New("fixture did not stop")
	}
}

type dnsTrap struct {
	conn    *net.UDPConn
	queries atomic.Int64
}

func startDNSTrap() (*dnsTrap, error) {
	address := &net.UDPAddr{IP: net.ParseIP("127.0.0.53"), Port: 53}
	conn, err := net.ListenUDP("udp4", address)
	if err != nil {
		return nil, err
	}
	trap := &dnsTrap{conn: conn}
	go func() {
		packet := make([]byte, 512)
		for {
			if _, _, err := conn.ReadFromUDP(packet); err != nil {
				return
			}
			trap.queries.Add(1)
		}
	}()
	return trap, nil
}

func (d *dnsTrap) close() int64 {
	_ = d.conn.Close()
	return d.queries.Load()
}

type tlsFront struct {
	listener net.Listener
	backend  string
	config   *tls.Config
	done     chan struct{}
	wg       sync.WaitGroup
}

func startTLSFront(endpointIP, backend string) (*tlsFront, string, string, error) {
	cert, der, err := makeCertificate()
	if err != nil {
		return nil, "", "", err
	}
	config := &tls.Config{Certificates: []tls.Certificate{cert}, MinVersion: tls.VersionTLS13}
	listener, err := tls.Listen("tcp4", net.JoinHostPort(endpointIP, "0"), config)
	if err != nil {
		return nil, "", "", err
	}
	front := &tlsFront{listener: listener, backend: backend, config: config, done: make(chan struct{})}
	front.wg.Add(1)
	go front.serve()
	digest := sha256.Sum256(der)
	return front, listener.Addr().String(), base64.StdEncoding.EncodeToString(digest[:]), nil
}

func makeCertificate() (tls.Certificate, []byte, error) {
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		return tls.Certificate{}, nil, err
	}
	serial, err := rand.Int(rand.Reader, new(big.Int).Lsh(big.NewInt(1), 128))
	if err != nil {
		return tls.Certificate{}, nil, err
	}
	template := x509.Certificate{
		SerialNumber: serial, Subject: pkix.Name{CommonName: "bridge.invalid"},
		DNSNames: []string{"bridge.invalid"}, NotBefore: time.Now().Add(-time.Hour),
		NotAfter: time.Now().Add(time.Hour), KeyUsage: x509.KeyUsageDigitalSignature,
		ExtKeyUsage: []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
	}
	der, err := x509.CreateCertificate(rand.Reader, &template, &template, &key.PublicKey, key)
	if err != nil {
		return tls.Certificate{}, nil, err
	}
	certPEM := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der})
	keyPEM := pem.EncodeToMemory(&pem.Block{Type: "RSA PRIVATE KEY", Bytes: x509.MarshalPKCS1PrivateKey(key)})
	cert, err := tls.X509KeyPair(certPEM, keyPEM)
	return cert, der, err
}

func (f *tlsFront) serve() {
	defer f.wg.Done()
	for {
		conn, err := f.listener.Accept()
		if err != nil {
			close(f.done)
			return
		}
		f.wg.Add(1)
		go func() {
			defer f.wg.Done()
			f.handle(conn)
		}()
	}
}

func (f *tlsFront) handle(client net.Conn) {
	defer client.Close()
	request, err := http.ReadRequest(bufio.NewReader(client))
	if err != nil {
		return
	}
	if request.URL.Path != webTunnelPath {
		response := &http.Response{StatusCode: http.StatusNotFound, ProtoMajor: 1, ProtoMinor: 1, Header: make(http.Header), Body: io.NopCloser(nilReader{})}
		_ = response.Write(client)
		return
	}
	backend, err := net.DialTimeout("tcp4", f.backend, time.Second)
	if err != nil {
		return
	}
	defer backend.Close()
	if err := request.Write(backend); err != nil {
		return
	}
	copyBoth(client, backend)
}

func (f *tlsFront) checkOrdinary(address string) error {
	config := f.config.Clone()
	// The exact DER certificate is checked independently through the candidate
	// pin. This probe exercises only the front's ordinary-path behavior.
	config.InsecureSkipVerify = true
	conn, err := tls.Dial("tcp4", address, config)
	if err != nil {
		return err
	}
	defer conn.Close()
	request, _ := http.NewRequest(http.MethodGet, "https://bridge.invalid/not-secret", nil)
	request.Host = "bridge.invalid"
	if err := request.Write(conn); err != nil {
		return err
	}
	response, err := http.ReadResponse(bufio.NewReader(conn), request)
	if err != nil {
		return err
	}
	_ = response.Body.Close()
	if response.StatusCode != http.StatusNotFound {
		return errors.New("ordinary path did not return bounded 404")
	}
	return nil
}

func (f *tlsFront) close() {
	_ = f.listener.Close()
	f.wg.Wait()
}

type nilReader struct{}

func (nilReader) Read([]byte) (int, error) { return 0, io.EOF }

func copyBoth(left, right net.Conn) {
	done := make(chan struct{}, 2)
	go func() { _, _ = io.Copy(left, right); done <- struct{}{} }()
	go func() { _, _ = io.Copy(right, left); done <- struct{}{} }()
	<-done
}

func reserveAddress() (string, error) {
	listener, err := net.Listen("tcp4", "127.0.0.1:0")
	if err != nil {
		return "", err
	}
	address := listener.Addr().String()
	return address, listener.Close()
}
