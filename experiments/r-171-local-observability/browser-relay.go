//go:build ignore

// Temporary browser probe tunnel. No product data or general proxy destination.
package main

import (
	"context"
	"flag"
	"io"
	"net"
	"os"
	"os/exec"
	"regexp"
	"sync"
	"time"
)

type idleConnection struct{ net.Conn }

func (c idleConnection) Read(buffer []byte) (int, error) {
	c.SetReadDeadline(time.Now().Add(30 * time.Second))
	return c.Conn.Read(buffer)
}

func (c idleConnection) Write(buffer []byte) (int, error) {
	c.SetWriteDeadline(time.Now().Add(30 * time.Second))
	return c.Conn.Write(buffer)
}

func main() {
	container := flag.String("container", "", "owned synthetic probe tunnel container")
	flag.Parse()
	if !regexp.MustCompile("^r171-[a-z0-9-]{1,32}-browser-tunnel-1$").MatchString(*container) {
		os.Exit(2)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 300*time.Second)
	defer cancel()
	listener, err := net.Listen("tcp4", "127.0.0.1:8098")
	if err != nil {
		os.Exit(1)
	}
	defer listener.Close()
	go func() { <-ctx.Done(); listener.Close() }()
	slots := make(chan struct{}, 8)
	var workers sync.WaitGroup
	for {
		client, err := listener.Accept()
		if err != nil {
			break
		}
		select {
		case slots <- struct{}{}:
		default:
			client.Close()
			continue
		}
		workers.Add(1)
		go func() {
			defer workers.Done()
			defer func() { <-slots }()
			defer client.Close()
			stream := idleConnection{client}
			script := "import socket,sys,threading\ns=socket.create_connection(('grafana',3000),5)\ns.settimeout(30)\ndef send():\n try:\n  while True:\n   b=sys.stdin.buffer.read1(65536)\n   if not b: break\n   s.sendall(b)\n except OSError: pass\n finally:\n  try: s.shutdown(socket.SHUT_WR)\n  except OSError: pass\nthreading.Thread(target=send,daemon=True).start()\ntry:\n while True:\n  b=s.recv(65536)\n  if not b: break\n  sys.stdout.buffer.write(b);sys.stdout.buffer.flush()\nfinally: s.close()\n"
			command := exec.CommandContext(ctx, "docker", "exec", "-i", *container, "python3", "-c", script)
			output, err := command.StdoutPipe()
			if err != nil {
				return
			}
			input, err := command.StdinPipe()
			if err != nil {
				return
			}
			command.Stderr = io.Discard
			if command.Start() != nil {
				input.Close()
				return
			}
			go func() { io.Copy(input, stream); input.Close() }()
			io.Copy(stream, output)
			client.Close()
			command.Wait()
		}()
	}
	cancel()
	workers.Wait()
}
