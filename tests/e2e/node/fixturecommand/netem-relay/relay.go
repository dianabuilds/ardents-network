package main

import (
	"fmt"
	"io"
	"net"
	"os"
	"os/exec"
	"os/signal"
	"sync"
	"syscall"
	"time"
)

const relayDirectionByteLimit = 1 << 20

func runRelay(configuration relayConfiguration) error {
	if err := applyNetem(configuration); err != nil {
		return err
	}
	listener, err := net.Listen("tcp", configuration.listen)
	if err != nil {
		return err
	}
	defer listener.Close()
	fmt.Println("netem-relay-ready")
	stopped := make(chan os.Signal, 1)
	signal.Notify(stopped, syscall.SIGINT, syscall.SIGTERM)
	defer signal.Stop(stopped)
	var work sync.WaitGroup
	go func() { <-stopped; _ = listener.Close() }()
	for {
		client, err := listener.Accept()
		if err != nil {
			break
		}
		work.Add(1)
		go func() { defer work.Done(); relayConnection(client, configuration.target) }()
	}
	work.Wait()
	return nil
}

func applyNetem(configuration relayConfiguration) error {
	command := exec.Command(configuration.tc, configuration.netemArguments()...)
	if output, err := command.CombinedOutput(); err != nil {
		return fmt.Errorf("apply netem: %w: %s", err, output)
	}
	return nil
}

func relayConnection(client net.Conn, target string) {
	defer client.Close()
	server, err := (&net.Dialer{Timeout: 4 * time.Second}).Dial("tcp", target)
	if err != nil {
		return
	}
	defer server.Close()
	done := make(chan struct{})
	go func() { _, _ = copyRelayDirection(server, client); close(done) }()
	_, _ = copyRelayDirection(client, server)
	_ = client.Close()
	_ = server.Close()
	<-done
}

func copyRelayDirection(destination net.Conn, source net.Conn) (int64, error) {
	return io.Copy(destination, io.LimitReader(source, relayDirectionByteLimit))
}
