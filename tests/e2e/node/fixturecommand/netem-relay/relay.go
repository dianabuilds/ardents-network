package main

import (
	"errors"
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
	traffic := &relayTraffic{}
	var err error
	if configuration.network == "udp" {
		err = runUDPRelay(configuration, traffic)
	} else {
		err = runTCPRelay(configuration, traffic)
	}
	return errors.Join(err, traffic.emit())
}

func runTCPRelay(configuration relayConfiguration, traffic *relayTraffic) error {
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
		go func() {
			defer work.Done()
			relayConnection(client, configuration.target, configuration.directionByteLimit(), traffic)
		}()
	}
	work.Wait()
	return nil
}

func applyNetem(configuration relayConfiguration) error {
	for _, arguments := range configuration.trafficControlCommands() {
		command := exec.Command(configuration.tc, arguments...)
		if output, err := command.CombinedOutput(); err != nil {
			return fmt.Errorf("apply traffic control: %w: %s", err, output)
		}
	}
	return nil
}

func relayConnection(client net.Conn, target string, limit int64, traffic *relayTraffic) {
	defer client.Close()
	server, err := (&net.Dialer{Timeout: 4 * time.Second}).Dial("tcp", target)
	if err != nil {
		return
	}
	defer server.Close()
	done := make(chan struct{})
	go func() { count, _ := copyRelayDirection(server, client, limit); traffic.add(true, count); close(done) }()
	count, _ := copyRelayDirection(client, server, limit)
	traffic.add(false, count)
	_ = client.Close()
	_ = server.Close()
	<-done
}

func copyRelayDirection(destination net.Conn, source net.Conn, limit int64) (int64, error) {
	return io.Copy(destination, io.LimitReader(source, limit))
}
