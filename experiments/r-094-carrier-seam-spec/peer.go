//go:build ignore

package main

import (
	"context"
	"errors"
	"io"
	"sync"

	"github.com/dianabuilds/ardents-network/internal/route"
)

const (
	requestBytes  = "r094-carrier-request"
	responseBytes = "r094-carrier-response"
	lossTrigger   = "!"
)

type peerMode byte

const (
	peerNormal peerMode = iota
	peerLossAfterBinding
	peerStallBinding
	peerStallData
)

type peerRuntime struct {
	Endpoint string
	stop     func()
	done     <-chan error
	once     sync.Once
	result   error
}

type closeSlot struct {
	mu    sync.Mutex
	close func()
}

func (runtime *peerRuntime) Close() error {
	runtime.once.Do(func() {
		runtime.stop()
		runtime.result = <-runtime.done
	})
	return runtime.result
}

func (slot *closeSlot) Set(close func()) {
	slot.mu.Lock()
	slot.close = close
	slot.mu.Unlock()
}

func (slot *closeSlot) Close() {
	slot.mu.Lock()
	close := slot.close
	slot.mu.Unlock()
	if close != nil {
		close()
	}
}

func servePeer(ctx context.Context, lane deadlineLane, binding route.LegBinding, mode peerMode) error {
	if mode == peerStallBinding {
		<-ctx.Done()
		return ctx.Err()
	}
	if err := route.AcceptNodeLegBinding(lane, binding); err != nil {
		return err
	}
	if mode == peerLossAfterBinding {
		trigger := make([]byte, len(lossTrigger))
		if _, err := io.ReadFull(lane, trigger); err != nil {
			return err
		}
		if string(trigger) != lossTrigger {
			return errors.New("peer received an invalid loss trigger")
		}
		return nil
	}
	if mode == peerStallData {
		<-ctx.Done()
		return ctx.Err()
	}
	received := make([]byte, len(requestBytes))
	if _, err := io.ReadFull(lane, received); err != nil {
		return err
	}
	if string(received) != requestBytes {
		return errors.New("peer received a noncanonical transcript")
	}
	_, err := lane.Write([]byte(responseBytes))
	return err
}

func joined(runtime *peerRuntime) bool {
	_ = runtime.Close()
	return true
}
