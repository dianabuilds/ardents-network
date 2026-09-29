package process

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"runtime/pprof"
	"runtime/trace"
	"strconv"
	"sync"
	"syscall"
	"time"
)

const maximumProfileBytes = 64 << 20

type limitedWriter struct {
	output    io.Writer
	remaining int64
	failure   error
	ctx       context.Context
}

func (writer *limitedWriter) Write(body []byte) (int, error) {
	if writer.ctx != nil && writer.ctx.Err() != nil {
		writer.failure = writer.ctx.Err()
		return 0, writer.failure
	}
	if int64(len(body)) > writer.remaining {
		writer.failure = errors.New("diagnostic profile exceeds byte bound")
		return 0, writer.failure
	}
	n, err := writer.output.Write(body)
	writer.remaining -= int64(n)
	if err != nil {
		writer.failure = err
	}
	if err == nil && n != len(body) {
		writer.failure = io.ErrShortWrite
		err = writer.failure
	}
	return n, err
}

type privateListener struct {
	net.Listener
	slots chan struct{}
}

func (listener privateListener) Accept() (net.Conn, error) {
	for {
		conn, err := listener.Listener.Accept()
		if err != nil {
			return nil, err
		}
		select {
		case listener.slots <- struct{}{}:
			return &privateConnection{Conn: conn, slots: listener.slots}, nil
		default:
			conn.Close()
		}
	}
}

type privateConnection struct {
	net.Conn
	slots chan struct{}
	once  sync.Once
}

func (conn *privateConnection) Close() error {
	err := conn.Conn.Close()
	conn.once.Do(func() { <-conn.slots })
	return err
}

func open(ctx context.Context, path string) (func() error, error) {
	if !filepath.IsAbs(path) || filepath.Clean(path) != path || len(path) > 100 {
		return nil, errors.New("diagnostic socket path must be absolute, canonical and at most 100 bytes")
	}
	parent := filepath.Dir(path)
	info, err := os.Lstat(parent)
	if err != nil {
		return nil, errors.New("diagnostic socket directory unavailable")
	}
	stat, ok := info.Sys().(*syscall.Stat_t)
	if !ok || !info.IsDir() || info.Mode().Perm() != 0700 || stat.Uid != uint32(os.Geteuid()) {
		return nil, errors.New("diagnostic socket directory must be owner-private")
	}
	resolved, err := filepath.EvalSymlinks(parent)
	if err != nil || resolved != parent {
		return nil, errors.New("diagnostic socket parent must not contain symlinks")
	}
	listener, err := net.ListenUnix("unix", &net.UnixAddr{Name: path, Net: "unix"})
	if err != nil {
		return nil, errors.New("diagnostic socket unavailable; existing path is never removed")
	}
	listener.SetUnlinkOnClose(false)
	if err := os.Chmod(path, 0600); err != nil {
		return nil, errors.Join(err, listener.Close(), os.Remove(path))
	}
	owned, err := os.Lstat(path)
	if err != nil {
		return nil, errors.Join(err, listener.Close())
	}
	runCtx, cancel := context.WithCancel(ctx)
	profiles := make(chan struct{}, 1)
	var cleanupMu sync.Mutex
	var cleanupFailure error
	retainCleanup := func(err error) {
		if err != nil {
			cleanupMu.Lock()
			if cleanupFailure == nil {
				cleanupFailure = err
			}
			cleanupMu.Unlock()
		}
	}

	mux := http.NewServeMux()
	mux.HandleFunc("GET /runtime", func(w http.ResponseWriter, r *http.Request) {
		var m runtime.MemStats
		runtime.ReadMemStats(&m)
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(struct {
			At          time.Time `json:"at"`
			Goroutines  int       `json:"goroutines"`
			HeapAlloc   uint64    `json:"heap_alloc_bytes"`
			HeapSys     uint64    `json:"heap_sys_bytes"`
			HeapObjects uint64    `json:"heap_objects"`
			NumGC       uint32    `json:"gc_cycles"`
			PauseTotal  uint64    `json:"gc_pause_total_ns"`
		}{time.Now().UTC(), runtime.NumGoroutine(), m.HeapAlloc, m.HeapSys, m.HeapObjects, m.NumGC, m.PauseTotalNs})
	})
	mux.HandleFunc("GET /profile/{kind}", func(w http.ResponseWriter, r *http.Request) {
		select {
		case profiles <- struct{}{}:
			defer func() { <-profiles }()
		default:
			http.Error(w, "diagnostic capture busy", http.StatusTooManyRequests)
			return
		}
		kind := r.PathValue("kind")
		if kind == "cpu" {
			captureTimed(r, w, runCtx, parent, retainCleanup, pprof.StartCPUProfile, pprof.StopCPUProfile)
			return
		}
		if kind != "heap" && kind != "allocs" && kind != "goroutine" && kind != "block" && kind != "mutex" {
			http.NotFound(w, r)
			return
		}
		profile := pprof.Lookup(kind)
		if profile == nil {
			http.Error(w, "diagnostic profile unavailable", http.StatusServiceUnavailable)
			return
		}
		file, err := openProfileFile(parent, retainCleanup)
		if err != nil {
			http.Error(w, "diagnostic storage unavailable", http.StatusServiceUnavailable)
			return
		}
		defer func() { retainCleanup(file.Close()) }()
		writer := &limitedWriter{output: file, remaining: maximumProfileBytes, ctx: r.Context()}
		if err := errors.Join(profile.WriteTo(writer, 0), writer.failure); err != nil {
			http.Error(w, "diagnostic profile incomplete", http.StatusServiceUnavailable)
			return
		}
		sendProfile(w, file)
	})
	mux.HandleFunc("GET /trace", func(w http.ResponseWriter, r *http.Request) {
		select {
		case profiles <- struct{}{}:
			defer func() { <-profiles }()
		default:
			http.Error(w, "diagnostic capture busy", http.StatusTooManyRequests)
			return
		}
		captureTimed(r, w, runCtx, parent, retainCleanup, trace.Start, trace.Stop)
	})
	var handlers sync.WaitGroup
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		handlers.Add(1)
		defer handlers.Done()
		mux.ServeHTTP(w, r)
	})
	server := &http.Server{Handler: handler, ReadHeaderTimeout: 2 * time.Second, ReadTimeout: 2 * time.Second, WriteTimeout: 35 * time.Second, IdleTimeout: 2 * time.Second, MaxHeaderBytes: 4096, BaseContext: func(net.Listener) context.Context { return runCtx }}
	runtime.SetBlockProfileRate(1_000_000)
	previousMutex := runtime.SetMutexProfileFraction(10)
	terminal := make(chan error, 1)
	go func() { terminal <- server.Serve(privateListener{Listener: listener, slots: make(chan struct{}, 4)}) }()
	var once sync.Once
	var result error
	return func() error {
		once.Do(func() {
			cancel()
			end, stop := context.WithTimeout(context.Background(), 3*time.Second)
			defer stop()
			result = server.Shutdown(end)
			if result != nil {
				result = errors.Join(result, server.Close())
			}
			joined := make(chan struct{})
			go func() { handlers.Wait(); close(joined) }()
			select {
			case <-joined:
			case <-end.Done():
				result = errors.Join(result, errors.New("diagnostic handlers did not join within shutdown budget"))
			}
			cleanupMu.Lock()
			result = errors.Join(result, cleanupFailure)
			cleanupMu.Unlock()
			serveErr := <-terminal
			if !errors.Is(serveErr, http.ErrServerClosed) {
				result = errors.Join(result, serveErr)
			}
			runtime.SetBlockProfileRate(0)
			runtime.SetMutexProfileFraction(previousMutex)
			current, err := os.Lstat(path)
			if err == nil && os.SameFile(owned, current) {
				result = errors.Join(result, os.Remove(path))
			} else if err != nil && !errors.Is(err, os.ErrNotExist) {
				result = errors.Join(result, err)
			}
		})
		return result
	}, nil
}

// Unlink before any sensitive write. The open descriptor remains seekable on
// Linux, while namespace residue cannot retain secret-bearing profile data.
func openProfileFile(parent string, retainCleanup func(error)) (*os.File, error) {
	file, err := os.CreateTemp(parent, ".profile-")
	if err != nil {
		return nil, err
	}
	if err := unlinkProfileFile(file, retainCleanup); err != nil {
		failure := errors.Join(err, file.Close())
		retainCleanup(failure)
		return nil, failure
	}
	return file, nil
}
func unlinkProfileFile(file *os.File, retainCleanup func(error)) error {
	err := os.Remove(file.Name())
	retainCleanup(err)
	return err
}

func captureTimed(r *http.Request, w http.ResponseWriter, ctx context.Context, parent string, retainCleanup func(error), start func(io.Writer) error, stop func()) {
	seconds, err := strconv.Atoi(r.URL.Query().Get("seconds"))
	if err != nil || seconds < 1 || seconds > 30 {
		http.Error(w, "seconds must be between 1 and 30", http.StatusBadRequest)
		return
	}
	file, err := openProfileFile(parent, retainCleanup)
	if err != nil {
		http.Error(w, "diagnostic storage unavailable", http.StatusServiceUnavailable)
		return
	}
	defer func() { retainCleanup(file.Close()) }()
	writer := &limitedWriter{output: file, remaining: maximumProfileBytes, ctx: r.Context()}
	if err := start(writer); err != nil {
		http.Error(w, "diagnostic recorder unavailable", http.StatusServiceUnavailable)
		return
	}
	timer := time.NewTimer(time.Duration(seconds) * time.Second)
	defer timer.Stop()
	select {
	case <-timer.C:
	case <-r.Context().Done():
	case <-ctx.Done():
	}
	stop()
	if writer.failure != nil || r.Context().Err() != nil || ctx.Err() != nil {
		http.Error(w, "diagnostic capture incomplete", http.StatusServiceUnavailable)
		return
	}
	sendProfile(w, file)
}

func sendProfile(w http.ResponseWriter, file *os.File) {
	if _, err := file.Seek(0, io.SeekStart); err != nil {
		http.Error(w, "diagnostic capture unavailable", http.StatusServiceUnavailable)
		return
	}
	info, err := file.Stat()
	if err != nil {
		http.Error(w, "diagnostic capture unavailable", http.StatusServiceUnavailable)
		return
	}
	w.Header().Set("Content-Type", "application/octet-stream")
	w.Header().Set("Content-Length", strconv.FormatInt(info.Size(), 10))
	io.Copy(w, file) // Content-Length makes a short transfer visible to the local client.
}
