package node

import (
	"context"
	"crypto/sha256"
	"crypto/tls"
	"crypto/x509"
	"errors"
	"net"
	"path/filepath"
	"runtime"
	"runtime/debug"
	"sync"
	"time"

	"github.com/dianabuilds/ardents-network/internal/qualification/byteio"
)

func runNodeInjection(ctx context.Context, input Campaign) Result {
	switch input.Injection {
	case "nofile":
		if err := lowerNodeNofile(); err != nil {
			return Result{Verdict: "fail", Reason: err.Error()}
		}
		return Result{Verdict: "pass", Reason: "candidate descriptor limit lowered"}
	case "memory":
		return injectNodeMemory(ctx)
	case "cpu":
		return injectNodeCPU(ctx)
	case "emfile":
		return injectNodeEMFILE(ctx, input.Addresses[0])
	case "probe":
		return injectNodeProbe(ctx, input)
	default:
		return Result{Verdict: "invalid", Reason: "node injection mode is invalid"}
	}
}

func injectNodeProbe(ctx context.Context, input Campaign) Result {
	if len(input.Addresses) != 2 {
		return Result{Verdict: "invalid", Reason: "node injector requires exactly two Node addresses"}
	}
	plan, err := readNodeProbePlan(input.ProbePlan)
	if err != nil {
		return Result{Verdict: "invalid", Reason: err.Error()}
	}
	for _, address := range input.Addresses {
		host, port, err := net.SplitHostPort(address)
		if err != nil || net.ParseIP(host) == nil || port == "" {
			return Result{Verdict: "invalid", Reason: "node injector address is invalid"}
		}
	}
	certificate, err := tls.LoadX509KeyPair(filepath.Join(input.SecretRoot, "harness-cert.pem"), filepath.Join(input.SecretRoot, "harness-key.pem"))
	if err != nil {
		return Result{Verdict: "invalid", Reason: err.Error()}
	}
	for index, rootName := range []string{"role-1-root.pem", "role-2-root.pem"} {
		address := input.Addresses[index]
		if plan.Nodes[index].Address != address {
			return Result{Verdict: "invalid", Reason: "node probe plan address does not match the isolated target"}
		}
		root, readErr := byteio.ReadFile(filepath.Join(input.SecretRoot, rootName), 64<<10)
		if readErr != nil {
			return Result{Verdict: "invalid", Reason: readErr.Error()}
		}
		pool := x509.NewCertPool()
		if !pool.AppendCertsFromPEM(root) {
			return Result{Verdict: "invalid", Reason: "node injector root is invalid"}
		}
		serverPin, pinErr := decodeNodeProbeDigest(plan.Nodes[index].ServerKeyDigest)
		if pinErr != nil {
			return Result{Verdict: "invalid", Reason: pinErr.Error()}
		}
		config := &tls.Config{MinVersion: tls.VersionTLS13, MaxVersion: tls.VersionTLS13,
			Certificates: []tls.Certificate{certificate}, RootCAs: pool, ServerName: "node.node", SessionTicketsDisabled: true,
			VerifyConnection: func(state tls.ConnectionState) error {
				if len(state.PeerCertificates) == 0 {
					return errors.New("node role server leaf is missing")
				}
				raw, err := x509.MarshalPKIXPublicKey(state.PeerCertificates[0].PublicKey)
				if err != nil || sha256.Sum256(raw) != serverPin {
					return errors.New("node role server leaf pin is invalid")
				}
				return nil
			}}
		if err := exerciseNodeProbe(ctx, address, config, plan, index); err != nil {
			return Result{Verdict: "fail", Reason: err.Error()}
		}
	}
	return Result{Verdict: "pass", Reason: "node probe faults remained bounded"}
}

func injectNodeMemory(ctx context.Context) Result {
	memory := make([]byte, 400<<20)
	for index := 0; index < len(memory); index += 4096 {
		memory[index] = byte(index)
	}
	timer := time.NewTimer(8 * time.Second)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		runtime.KeepAlive(memory)
		return Result{Verdict: "fail", Reason: ctx.Err().Error()}
	case <-timer.C:
		runtime.KeepAlive(memory)
		memory = nil
		runtime.GC()
		debug.FreeOSMemory()
		return Result{Verdict: "pass", Reason: "bounded memory pressure completed"}
	}
}

func injectNodeCPU(ctx context.Context) Result {
	runtime.GOMAXPROCS(2)
	stop := make(chan struct{})
	var work sync.WaitGroup
	for range 2 {
		work.Add(1)
		go func() {
			defer work.Done()
			value := uint64(1)
			for {
				select {
				case <-stop:
					runtime.KeepAlive(value)
					return
				default:
					value = value*6364136223846793005 + 1
				}
			}
		}()
	}
	timer := time.NewTimer(8 * time.Second)
	select {
	case <-ctx.Done():
	case <-timer.C:
	}
	if !timer.Stop() {
		select {
		case <-timer.C:
		default:
		}
	}
	close(stop)
	work.Wait()
	if ctx.Err() != nil {
		return Result{Verdict: "fail", Reason: ctx.Err().Error()}
	}
	return Result{Verdict: "pass", Reason: "bounded CPU pressure completed"}
}

func injectNodeEMFILE(ctx context.Context, address string) Result {
	connections := make([]net.Conn, 0, 64)
	defer func() {
		for _, connection := range connections {
			_ = connection.Close()
		}
	}()
	for range 64 {
		connection, err := (&net.Dialer{Timeout: time.Second}).DialContext(ctx, "tcp", address)
		if err == nil {
			connections = append(connections, connection)
		}
	}
	if len(connections) == 0 {
		return Result{Verdict: "fail", Reason: "EMFILE injector could not establish a descriptor stimulus"}
	}
	if err := waitNode(ctx, 5*time.Second); err != nil {
		return Result{Verdict: "fail", Reason: err.Error()}
	}
	return Result{Verdict: "pass", Reason: "EMFILE descriptor occupancy completed"}
}

func exerciseNodeProbe(ctx context.Context, address string, config *tls.Config, plan nodeProbePlan, node int) error {
	deadline := nodeProbeDeadline(ctx)
	unauthenticated, err := tls.DialWithDialer(&net.Dialer{Deadline: deadline}, "tcp", address,
		&tls.Config{MinVersion: tls.VersionTLS13, MaxVersion: tls.VersionTLS13, InsecureSkipVerify: true}) // qualification-only negative
	if err == nil {
		_ = unauthenticated.SetDeadline(time.Now().Add(time.Second))
		_, _ = unauthenticated.Write([]byte("ARNP"))
		var response [1]byte
		_, rejectErr := unauthenticated.Read(response[:])
		_ = unauthenticated.Close()
		if rejectErr == nil {
			return errors.New("role probe returned data to a TLS client without a certificate")
		}
		if timeout, ok := rejectErr.(net.Error); ok && timeout.Timeout() {
			return errors.New("role probe did not reject a TLS client without a certificate")
		}
	}
	if err := runNodeProbeMatrix(address, config, plan, node, nodeProbeDeadline(ctx)); err != nil {
		return err
	}
	if err := partialNodeProbe(address, config, nodeProbeDeadline(ctx)); err != nil {
		return err
	}
	if err := slowNodeProbe(address, nodeProbeDeadline(ctx)); err != nil {
		return err
	}
	return floodNodeProbe(ctx, address, config)
}

func nodeProbeDeadline(ctx context.Context) time.Time {
	deadline := time.Now().Add(8 * time.Second)
	if limit, ok := ctx.Deadline(); ok && limit.Before(deadline) {
		return limit
	}
	return deadline
}
