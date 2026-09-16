package main

import (
	"errors"
	"fmt"
	"net"
	"os"
	"os/signal"
	"sync"
	"sync/atomic"
	"syscall"
)

const maximumUDPSessions = 512

type udpRelaySession struct {
	client         *net.UDPAddr
	upstream       *net.UDPConn
	towardUpstream atomic.Int64
	towardClient   atomic.Int64
}

func runUDPRelay(configuration relayConfiguration, traffic *relayTraffic) error {
	listen, err := net.ResolveUDPAddr("udp", configuration.listen)
	if err != nil {
		return err
	}
	target, err := net.ResolveUDPAddr("udp", configuration.target)
	if err != nil {
		return err
	}
	listener, err := net.ListenUDP("udp", listen)
	if err != nil {
		return err
	}
	defer listener.Close()
	fmt.Println("netem-relay-ready")
	stopped := make(chan os.Signal, 1)
	signal.Notify(stopped, syscall.SIGINT, syscall.SIGTERM)
	defer signal.Stop(stopped)
	go func() { <-stopped; _ = listener.Close() }()
	sessions := make(map[string]*udpRelaySession, maximumUDPSessions)
	var work sync.WaitGroup
	defer func() {
		for _, session := range sessions {
			_ = session.upstream.Close()
		}
		work.Wait()
	}()
	buffer := make([]byte, 64<<10)
	for {
		count, client, readErr := listener.ReadFromUDP(buffer)
		if readErr != nil {
			if errors.Is(readErr, net.ErrClosed) {
				return nil
			}
			return readErr
		}
		key := client.String()
		session := sessions[key]
		if session == nil {
			if len(sessions) >= maximumUDPSessions {
				continue
			}
			upstream, dialErr := net.DialUDP("udp", nil, target)
			if dialErr != nil {
				continue
			}
			session = &udpRelaySession{client: client, upstream: upstream}
			sessions[key] = session
			work.Add(1)
			go func() {
				defer work.Done()
				response := make([]byte, 64<<10)
				for {
					size, responseErr := session.upstream.Read(response)
					if responseErr != nil {
						return
					}
					if session.towardClient.Add(int64(size)) > configuration.directionByteLimit() {
						return
					}
					if _, writeErr := listener.WriteToUDP(response[:size], session.client); writeErr != nil {
						return
					}
					traffic.add(false, int64(size))
				}
			}()
		}
		if session.towardUpstream.Add(int64(count)) > configuration.directionByteLimit() {
			continue
		}
		if written, writeErr := session.upstream.Write(buffer[:count]); writeErr == nil {
			traffic.add(true, int64(written))
		}
	}
}
