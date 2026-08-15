package tooling

import (
	"flag"
	"fmt"
	"io"
	"net"
	"os"
	"runtime"
	"time"
)

const (
	minimumPressureMemory = 1 << 20
	maximumPressureMemory = 480 << 20
)

// RunMemoryPressure holds a bounded resident allocation for live qualification.
func RunMemoryPressure(arguments []string) error {
	flags := flag.NewFlagSet("pressure-memory", flag.ContinueOnError)
	flags.SetOutput(io.Discard)
	bytes := flags.Int64("bytes", 0, "resident bytes to hold")
	duration := flags.Duration("duration", 0, "bounded hold duration")
	connect := flags.String("connect", "", "optional incomplete TCP setup endpoint")
	if err := flags.Parse(arguments); err != nil || flags.NArg() != 0 ||
		*bytes < minimumPressureMemory || *bytes > maximumPressureMemory ||
		*duration < time.Second || *duration > time.Minute {
		return fmt.Errorf("pressure-memory requires bounded bytes and duration")
	}
	if *connect != "" {
		if _, _, err := net.SplitHostPort(*connect); err != nil {
			return fmt.Errorf("pressure-memory connect endpoint is invalid")
		}
	}
	var connection net.Conn
	if *connect != "" {
		var err error
		connection, err = net.DialTimeout("tcp", *connect, time.Second)
		if err != nil {
			return fmt.Errorf("pressure-memory connect: %w", err)
		}
		defer connection.Close()
	}
	memory := make([]byte, int(*bytes))
	for offset := 0; offset < len(memory); offset += os.Getpagesize() {
		memory[offset] = byte(offset)
	}
	timer := time.NewTimer(*duration)
	defer timer.Stop()
	<-timer.C
	runtime.KeepAlive(memory)
	return nil
}
