//go:build ignore

// R-096 is a disposable existing-browser loopback presentation fixture.
package main

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"os/signal"
	"path/filepath"
	"sync"
	"syscall"
	"time"
)

const (
	maxPublisherResponse = 128 << 10
	staticCSP            = "default-src 'self'; script-src 'none'; connect-src 'none'; img-src 'self'; style-src 'self'; object-src 'none'; base-uri 'none'; form-action 'none'; frame-ancestors 'none'; worker-src 'none'"
)

type readyRecord struct {
	Schema             string `json:"schema"`
	URL                string `json:"url"`
	Origin             string `json:"origin"`
	AdapterListener    string `json:"adapter_listener"`
	CapabilityBits     int    `json:"path_capability_bits"`
	CapabilitySHA256   string `json:"path_capability_sha256"`
	PublisherSimulator bool   `json:"publisher_simulator"`
	ExternalSentinel   bool   `json:"external_request_sentinel"`
	NavigationProbe    bool   `json:"navigation_probe"`
	CSPSandbox         bool   `json:"csp_sandbox"`
	ProcessID          int    `json:"process_id"`
}

type observedRequest struct {
	Schema    string `json:"schema"`
	Sequence  uint64 `json:"sequence"`
	Component string `json:"component"`
	Method    string `json:"method"`
	Path      string `json:"path"`
	Host      string `json:"host"`
	Status    int    `json:"status"`
	UserAgent string `json:"user_agent,omitempty"`
}

type eventRecorder struct {
	mu       sync.Mutex
	file     *os.File
	sequence uint64
}

type statusRecorder struct {
	http.ResponseWriter
	status int
}

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func run() error {
	readyPath := flag.String("ready", "", "fresh absolute file to receive readiness JSON")
	eventsPath := flag.String("events", "", "fresh absolute file to receive request JSONL")
	navigationProbe := flag.Bool("navigation-probe", false, "add external meta-refresh and link probes")
	cspSandbox := flag.Bool("csp-sandbox", true, "include sandbox allow-same-origin in the CSP")
	flag.Parse()
	if err := validateOutputPath("ready", *readyPath); err != nil {
		return err
	}
	if err := validateOutputPath("events", *eventsPath); err != nil {
		return err
	}
	if filepath.Clean(*readyPath) == filepath.Clean(*eventsPath) {
		return errors.New("ready and events paths must differ")
	}

	recorder, err := newEventRecorder(*eventsPath)
	if err != nil {
		return fmt.Errorf("create event log: %w", err)
	}
	defer recorder.close()

	externalListener, err := net.Listen("tcp4", "127.0.0.1:0")
	if err != nil {
		return fmt.Errorf("listen for external-request sentinel: %w", err)
	}
	defer externalListener.Close()
	_, externalPort, err := net.SplitHostPort(externalListener.Addr().String())
	if err != nil {
		return fmt.Errorf("inspect external-request sentinel address: %w", err)
	}
	externalURL := "http://localhost:" + externalPort + "/r096-external.svg"
	externalServer := &http.Server{
		Handler:           observedHandler(recorder, "external-sentinel", externalSentinelHandler()),
		ReadHeaderTimeout: 2 * time.Second,
		IdleTimeout:       5 * time.Second,
	}
	adapterCSP := staticCSP
	if *cspSandbox {
		adapterCSP += "; sandbox allow-same-origin"
	}

	publisherListener, err := net.Listen("tcp4", "127.0.0.1:0")
	if err != nil {
		return fmt.Errorf("listen for publisher simulator: %w", err)
	}
	defer publisherListener.Close()
	publisherServer := &http.Server{
		Handler:           observedHandler(recorder, "publisher", publisherHandler(externalURL, *navigationProbe)),
		ReadHeaderTimeout: 2 * time.Second,
		IdleTimeout:       5 * time.Second,
	}

	capability, err := randomCapability()
	if err != nil {
		return err
	}
	adapterListener, err := net.Listen("tcp4", "127.0.0.1:0")
	if err != nil {
		return fmt.Errorf("listen for browser Adapter: %w", err)
	}
	defer adapterListener.Close()
	publisherBase := "http://" + publisherListener.Addr().String()
	adapterServer := &http.Server{
		Handler:           observedHandler(recorder, "adapter", adapterHandler(capability, publisherBase, adapterCSP)),
		ReadHeaderTimeout: 2 * time.Second,
		IdleTimeout:       5 * time.Second,
	}

	serveResults := make(chan error, 3)
	go serve("external-request sentinel", externalServer, externalListener, serveResults)
	go serve("publisher simulator", publisherServer, publisherListener, serveResults)
	go serve("browser Adapter", adapterServer, adapterListener, serveResults)

	origin := "http://" + adapterListener.Addr().String()
	pageURL := origin + "/site/" + capability + "/"
	hash := sha256.Sum256([]byte(capability))
	ready := readyRecord{
		Schema:             "ardents-r096-browser-ready-v1",
		URL:                pageURL,
		Origin:             origin,
		AdapterListener:    adapterListener.Addr().String(),
		CapabilityBits:     128,
		CapabilitySHA256:   hex.EncodeToString(hash[:]),
		PublisherSimulator: true,
		ExternalSentinel:   true,
		NavigationProbe:    *navigationProbe,
		CSPSandbox:         *cspSandbox,
		ProcessID:          os.Getpid(),
	}
	if err := writeExclusiveJSON(*readyPath, ready); err != nil {
		shutdownServers(externalServer, publisherServer, adapterServer)
		return fmt.Errorf("write readiness: %w", err)
	}
	encodedReady, _ := json.Marshal(ready)
	fmt.Println(string(encodedReady))

	signals := make(chan os.Signal, 1)
	signal.Notify(signals, os.Interrupt, syscall.SIGTERM)
	defer signal.Stop(signals)
	var firstErr error
	joined := 0
	select {
	case signal := <-signals:
		fmt.Fprintln(os.Stderr, "stopping after signal:", signal)
	case firstErr = <-serveResults:
		joined = 1
	}
	shutdownServers(externalServer, publisherServer, adapterServer)
	for joined < 3 {
		select {
		case err := <-serveResults:
			joined++
			if firstErr == nil {
				firstErr = err
			}
		case <-time.After(3 * time.Second):
			if firstErr == nil {
				firstErr = errors.New("server did not join after shutdown")
			}
			joined = 3
		}
	}
	return firstErr
}

func validateOutputPath(name, path string) error {
	if path == "" {
		return fmt.Errorf("missing -%s", name)
	}
	if !filepath.IsAbs(path) {
		return fmt.Errorf("-%s must be an absolute path", name)
	}
	if _, err := os.Lstat(path); err == nil {
		return fmt.Errorf("-%s file already exists", name)
	} else if !os.IsNotExist(err) {
		return fmt.Errorf("inspect -%s file: %w", name, err)
	}
	parent := filepath.Dir(path)
	if info, err := os.Stat(parent); err != nil {
		return fmt.Errorf("inspect -%s parent: %w", name, err)
	} else if !info.IsDir() {
		return fmt.Errorf("-%s parent is not a directory", name)
	}
	return nil
}

func newEventRecorder(path string) (*eventRecorder, error) {
	file, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
	if err != nil {
		return nil, err
	}
	return &eventRecorder{file: file}, nil
}

func (recorder *eventRecorder) close() {
	recorder.mu.Lock()
	defer recorder.mu.Unlock()
	_ = recorder.file.Close()
}

func (recorder *eventRecorder) append(component string, request *http.Request, status int) {
	recorder.mu.Lock()
	defer recorder.mu.Unlock()
	recorder.sequence++
	event := observedRequest{
		Schema:    "ardents-r096-browser-request-v1",
		Sequence:  recorder.sequence,
		Component: component,
		Method:    request.Method,
		Path:      request.URL.EscapedPath(),
		Host:      request.Host,
		Status:    status,
	}
	if component == "adapter" {
		event.UserAgent = request.UserAgent()
	}
	if err := json.NewEncoder(recorder.file).Encode(event); err != nil {
		fmt.Fprintln(os.Stderr, "record request:", err)
		return
	}
	if err := recorder.file.Sync(); err != nil {
		fmt.Fprintln(os.Stderr, "sync request log:", err)
	}
}

func (writer *statusRecorder) WriteHeader(status int) {
	if writer.status != 0 {
		return
	}
	writer.status = status
	writer.ResponseWriter.WriteHeader(status)
}

func (writer *statusRecorder) Write(body []byte) (int, error) {
	if writer.status == 0 {
		writer.WriteHeader(http.StatusOK)
	}
	return writer.ResponseWriter.Write(body)
}

func observedHandler(recorder *eventRecorder, component string, next http.Handler) http.Handler {
	return http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		observed := &statusRecorder{ResponseWriter: response}
		next.ServeHTTP(observed, request)
		if observed.status == 0 {
			observed.status = http.StatusOK
		}
		recorder.append(component, request, observed.status)
	})
}

func externalSentinelHandler() http.Handler {
	return http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		response.Header().Set("Content-Type", "image/svg+xml")
		response.WriteHeader(http.StatusOK)
		if request.Method != http.MethodHead {
			_, _ = io.WriteString(response, `<svg xmlns="http://www.w3.org/2000/svg" width="8" height="8"><rect width="8" height="8" fill="#f00"/></svg>`)
		}
	})
}

func publisherHandler(externalURL string, navigationProbe bool) http.Handler {
	return http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		if request.Method != http.MethodGet && request.Method != http.MethodHead {
			response.Header().Set("Allow", "GET, HEAD")
			http.Error(response, "publisher method rejected", http.StatusMethodNotAllowed)
			return
		}
		response.Header().Set("Set-Cookie", "publisher-cookie=must-not-cross")
		response.Header().Set("Location", "https://invalid.example/must-not-cross")
		switch request.URL.Path {
		case "/":
			response.Header().Set("Content-Type", "text/html; charset=utf-8")
			headProbe := ""
			bodyProbe := ""
			if navigationProbe {
				headProbe = `<meta http-equiv="refresh" content="0;url=` + externalURL + `">`
				bodyProbe = `<a id="external-link" href="` + externalURL + `">external navigation probe</a>`
			}
			body := `<!doctype html><html><head><meta charset="utf-8"><title>Ardents R-096 fixture</title>` + headProbe + `<link rel="icon" href="visual.svg"><link rel="stylesheet" href="resource.css"></head><body><h1 id="title">Ardents static fixture</h1><img id="same-service-image" alt="same service" src="visual.svg"><img id="external-image" alt="external blocked" src="` + externalURL + `"><script>window.inlineEscapeRan=true</script><p id="same-service">same-service only</p>` + bodyProbe + `</body></html>`
			_, _ = io.WriteString(response, body)
		case "/resource.css":
			response.Header().Set("Content-Type", "text/css; charset=utf-8")
			_, _ = io.WriteString(response, "#same-service { color: rgb(1, 2, 3); }")
		case "/visual.svg":
			response.Header().Set("Content-Type", "image/svg+xml")
			_, _ = io.WriteString(response, `<svg xmlns="http://www.w3.org/2000/svg" width="8" height="8"><rect width="8" height="8" fill="#0a7"/></svg>`)
		default:
			http.NotFound(response, request)
		}
	})
}

func adapterHandler(capability, publisherBase, csp string) http.Handler {
	transport := &http.Transport{
		Proxy:             nil,
		DisableKeepAlives: true,
	}
	client := &http.Client{
		Transport: transport,
		Timeout:   3 * time.Second,
		CheckRedirect: func(_ *http.Request, _ []*http.Request) error {
			return errors.New("publisher redirect rejected")
		},
	}
	root := "/site/" + capability + "/"
	routes := map[string]string{
		root:                  "/",
		root + "resource.css": "/resource.css",
		root + "visual.svg":   "/visual.svg",
	}
	return http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		setAdapterPolicy(response.Header(), csp)
		if request.Method != http.MethodGet && request.Method != http.MethodHead {
			response.Header().Set("Allow", "GET, HEAD")
			http.Error(response, "Adapter method rejected", http.StatusMethodNotAllowed)
			return
		}
		publisherPath, allowed := routes[request.URL.EscapedPath()]
		if !allowed || request.URL.RawQuery != "" {
			http.NotFound(response, request)
			return
		}
		if err := forwardStatic(response, request, client, publisherBase, publisherPath); err != nil {
			http.Error(response, "Publisher simulator unavailable", http.StatusBadGateway)
		}
	})
}

func setAdapterPolicy(header http.Header, csp string) {
	header.Set("Cache-Control", "no-store")
	header.Set("Content-Security-Policy", csp)
	header.Set("Cross-Origin-Opener-Policy", "same-origin")
	header.Set("Referrer-Policy", "no-referrer")
	header.Set("X-Content-Type-Options", "nosniff")
}

func forwardStatic(response http.ResponseWriter, browserRequest *http.Request, client *http.Client, publisherBase, publisherPath string) error {
	target, err := url.JoinPath(publisherBase, publisherPath)
	if err != nil {
		return err
	}
	request, err := http.NewRequestWithContext(browserRequest.Context(), browserRequest.Method, target, nil)
	if err != nil {
		return err
	}
	publisherResponse, err := client.Do(request)
	if err != nil {
		return err
	}
	defer publisherResponse.Body.Close()
	if publisherResponse.StatusCode != http.StatusOK {
		return fmt.Errorf("publisher returned %d", publisherResponse.StatusCode)
	}
	body, err := io.ReadAll(io.LimitReader(publisherResponse.Body, maxPublisherResponse+1))
	if err != nil {
		return err
	}
	if len(body) > maxPublisherResponse {
		return errors.New("publisher response exceeded fixture limit")
	}
	contentType := publisherResponse.Header.Get("Content-Type")
	if contentType == "" {
		return errors.New("publisher response omitted content type")
	}
	response.Header().Set("Content-Type", contentType)
	response.Header().Set("Content-Length", fmt.Sprint(len(body)))
	response.WriteHeader(http.StatusOK)
	if browserRequest.Method == http.MethodHead {
		return nil
	}
	_, err = response.Write(body)
	return err
}

func randomCapability() (string, error) {
	random := make([]byte, 16)
	if _, err := rand.Read(random); err != nil {
		return "", fmt.Errorf("generate path capability: %w", err)
	}
	return hex.EncodeToString(random), nil
}

func writeExclusiveJSON(path string, value any) error {
	file, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
	if err != nil {
		return err
	}
	encoderErr := json.NewEncoder(file).Encode(value)
	syncErr := file.Sync()
	closeErr := file.Close()
	return errors.Join(encoderErr, syncErr, closeErr)
}

func serve(name string, server *http.Server, listener net.Listener, results chan<- error) {
	err := server.Serve(listener)
	if errors.Is(err, http.ErrServerClosed) {
		err = nil
	}
	if err != nil {
		err = fmt.Errorf("%s: %w", name, err)
	}
	results <- err
}

func shutdownServers(servers ...*http.Server) {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	for _, server := range servers {
		_ = server.Shutdown(ctx)
	}
}
