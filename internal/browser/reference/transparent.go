package browserreference

import (
	"bufio"
	"context"
	"errors"
	"io"
	"net"
	"net/http"
	"sync"
	"time"
)

const (
	transparentMaximumHeaderBytes       = 16 << 10
	transparentServerMaxHeaderBytes     = transparentMaximumHeaderBytes - (4 << 10)
	transparentMaximumBodyBytes         = 1 << 20
	transparentReadHeaderTimeout        = time.Second
	transparentIdleTimeout              = 5 * time.Second
	transparentResponseHeaderLimitError = "transparent Service response exceeds the alpha header limit"
)

// TransparentConfig binds one exact alpha HTTP name to an already
// authenticated Service Connection. The connection is the only destination;
// this local presentation cannot select a Target, dial a host, or interpret
// Publisher application content.
type TransparentConfig struct {
	// Target is retained only for historical compatibility readers. The
	// Browser Adapter never receives it; presentation security comes from the
	// already-authenticated local Application Connection.
	Target     [32]byte
	Hostname   string
	Connection io.ReadWriteCloser
}

// TransparentServer owns the local side of one HTTP/1.1 Service presentation.
// It preserves HTTP request and response semantics over its one selected,
// ordered Service Connection. Concurrent browser requests queue on that
// connection instead of opening another destination.
type TransparentServer struct {
	listener   net.Listener
	server     *http.Server
	hostname   string
	connection io.ReadWriteCloser
	reader     *bufio.Reader

	requestMu sync.Mutex
	work      sync.WaitGroup
	closeOnce sync.Once
	closeErr  error
}

// OpenTransparent starts one loopback-only alpha origin. It does not open a
// browser, perform Name resolution, or make the visible HTTP name public DNS.
func OpenTransparent(config TransparentConfig) (*TransparentServer, error) {
	if !validAlphaHTTPHost(config.Hostname) || config.Connection == nil {
		return nil, errors.New("transparent Service presentation is invalid")
	}
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return nil, err
	}
	running := &TransparentServer{listener: listener, hostname: config.Hostname, connection: config.Connection,
		reader: bufio.NewReader(config.Connection)}
	// net/http reserves 4 KiB above MaxHeaderBytes for its bufio reader. Set the
	// underlying value below the product ceiling so a Browser request head never
	// exceeds the declared 16 KiB alpha bound.
	running.server = &http.Server{Handler: http.HandlerFunc(running.serve), ReadHeaderTimeout: transparentReadHeaderTimeout,
		IdleTimeout: transparentIdleTimeout, MaxHeaderBytes: transparentServerMaxHeaderBytes}
	running.work.Add(1)
	go func() {
		defer running.work.Done()
		_ = running.server.Serve(listener)
	}()
	return running, nil
}

// Close withdraws the one visible name and closes its selected Service
// Connection. It never leaves a local listener that could be retargeted.
func (server *TransparentServer) Close() error {
	if server == nil || server.server == nil {
		return nil
	}
	server.closeOnce.Do(func() {
		server.requestMu.Lock()
		server.closeErr = errors.Join(server.closeErr, server.connection.Close())
		server.requestMu.Unlock()
		shutdown, cancel := context.WithTimeout(context.Background(), time.Second)
		defer cancel()
		if err := server.server.Shutdown(shutdown); err != nil {
			closeErr := server.server.Close()
			if closeErr != nil && !errors.Is(closeErr, http.ErrServerClosed) {
				server.closeErr = errors.Join(server.closeErr, err, closeErr)
			} else {
				server.closeErr = errors.Join(server.closeErr, err)
			}
		}
		server.work.Wait()
	})
	return server.closeErr
}

func (server *TransparentServer) alphaOriginAddress() string {
	if server == nil || server.listener == nil {
		return ""
	}
	return server.listener.Addr().String()
}

func (server *TransparentServer) alphaOriginHost(visibleHost string) string {
	if server == nil || visibleHost != server.hostname {
		return ""
	}
	return server.hostname
}

func (server *TransparentServer) alphaOriginPath(path string) string { return path }

func (server *TransparentServer) serve(writer http.ResponseWriter, request *http.Request) {
	if request.Method == http.MethodConnect || request.Host != server.hostname || request.URL.IsAbs() || request.URL.User != nil ||
		request.Header.Get("Upgrade") != "" {
		http.Error(writer, "invalid transparent Service request", http.StatusBadRequest)
		return
	}
	if request.ContentLength > transparentMaximumBodyBytes {
		http.Error(writer, "transparent Service request exceeds the alpha body limit", http.StatusRequestEntityTooLarge)
		return
	}
	request.Body = http.MaxBytesReader(writer, request.Body, transparentMaximumBodyBytes)
	if err := server.forward(request, writer); err != nil {
		if transparentRequestExceedsBodyLimit(err) {
			_ = server.connection.Close()
			http.Error(writer, "transparent Service request exceeds the alpha body limit", http.StatusRequestEntityTooLarge)
			return
		}
		http.Error(writer, "alpha service unavailable", http.StatusBadGateway)
	}
}

// transparentRequestExceedsBodyLimit also accepts net/http's internal
// request-body write wrapper: it does not expose Unwrap but preserves the
// documented MaxBytesError text.
func transparentRequestExceedsBodyLimit(err error) bool {
	var tooLarge *http.MaxBytesError
	return errors.As(err, &tooLarge) || (err != nil && err.Error() == "http: request body too large")
}

func (server *TransparentServer) forward(request *http.Request, writer http.ResponseWriter) error {
	server.requestMu.Lock()
	defer server.requestMu.Unlock()
	stop := context.AfterFunc(request.Context(), func() { _ = server.connection.Close() })
	defer stop()
	forward := request.Clone(request.Context())
	forward.RequestURI = ""
	forward.Header = cloneForwardHeaders(request.Header)
	if err := forward.Write(server.connection); err != nil {
		_ = server.connection.Close()
		return err
	}
	responseReader := bufio.NewReader(&transparentResponseHeaderReader{source: server.reader, maximum: transparentMaximumHeaderBytes})
	response, err := http.ReadResponse(responseReader, forward)
	if err != nil {
		_ = server.connection.Close()
		return err
	}
	defer response.Body.Close()
	if response.StatusCode == http.StatusSwitchingProtocols || response.Header.Get("Upgrade") != "" {
		_ = server.connection.Close()
		return errors.New("transparent Service upgrade is unavailable")
	}
	if response.ContentLength > transparentMaximumBodyBytes {
		_ = server.connection.Close()
		return errors.New("transparent Service response exceeds the alpha body limit")
	}
	copyForwardHeaders(writer.Header(), response.Header)
	writer.WriteHeader(response.StatusCode)
	if request.Method == http.MethodHead {
		return nil
	}
	if err := copyAndFlushBounded(writer, io.LimitReader(response.Body, transparentMaximumBodyBytes+1), transparentMaximumBodyBytes); err != nil {
		_ = server.connection.Close()
	}
	// Once headers are committed, a stream failure must close this selected
	// Service Connection rather than write a second, misleading HTTP result.
	return nil
}

type transparentResponseHeaderReader struct {
	source   io.Reader
	maximum  int
	seen     int
	tail     [4]byte
	tailLen  int
	complete bool
}

func (reader *transparentResponseHeaderReader) Read(destination []byte) (int, error) {
	read, readErr := reader.source.Read(destination)
	if reader.complete || read == 0 {
		return read, readErr
	}
	for index := 0; index < read; index++ {
		reader.seen++
		if reader.tailLen < len(reader.tail) {
			reader.tail[reader.tailLen] = destination[index]
			reader.tailLen++
		} else {
			copy(reader.tail[:], reader.tail[1:])
			reader.tail[len(reader.tail)-1] = destination[index]
		}
		if reader.tailLen == len(reader.tail) && reader.tail == [4]byte{'\r', '\n', '\r', '\n'} {
			reader.complete = true
			return read, readErr
		}
		if reader.seen >= reader.maximum {
			return 0, errors.New(transparentResponseHeaderLimitError)
		}
	}
	return read, readErr
}

func copyAndFlushBounded(writer http.ResponseWriter, source io.Reader, maximum int64) error {
	var copied int64
	buffer := make([]byte, 32<<10)
	controller := http.NewResponseController(writer)
	for {
		read, readErr := source.Read(buffer)
		if read > 0 {
			copied += int64(read)
			if copied > maximum {
				return errors.New("transparent Service response exceeds the alpha body limit")
			}
			if _, err := writer.Write(buffer[:read]); err != nil {
				return err
			}
			if err := controller.Flush(); err != nil {
				return err
			}
		}
		if readErr == io.EOF {
			return nil
		}
		if readErr != nil {
			return readErr
		}
	}
}

func copyAndFlush(writer http.ResponseWriter, source io.Reader) error {
	buffer := make([]byte, 32<<10)
	controller := http.NewResponseController(writer)
	for {
		read, readErr := source.Read(buffer)
		if read > 0 {
			if _, err := writer.Write(buffer[:read]); err != nil {
				return err
			}
			if err := controller.Flush(); err != nil {
				return err
			}
		}
		if readErr == io.EOF {
			return nil
		}
		if readErr != nil {
			return readErr
		}
	}
}
