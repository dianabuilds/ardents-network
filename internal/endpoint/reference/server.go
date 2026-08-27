package reference

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"io"
	"net"
	"net/http"
	"path"
	"strconv"
	"strings"
	"sync"
	"time"
)

const contentPolicy = "sandbox allow-same-origin; default-src 'none'; script-src 'none'; connect-src 'none'; img-src 'self'; style-src 'self'; object-src 'none'; base-uri 'none'; form-action 'none'; frame-ancestors 'none'; worker-src 'none'"

// Resource is one declared same-origin static Reference Site resource.
type Resource struct {
	ContentType string
	Body        []byte
}

// Config binds one pre-authenticated Target to a finite static presentation.
// Target has already been chosen by Endpoint; this package has no destination
// parsing or remote connection authority.
type Config struct {
	Target    [32]byte
	Document  Resource
	Resources map[string]Resource
}

// Server owns one loopback listener and its single opaque presentation path.
// Closing it withdraws the origin rather than retargeting requests elsewhere.
type Server struct {
	listener   net.Listener
	basePath   string
	originHost string
	target     [32]byte
	document   Resource
	resources  map[string]Resource
	routes     map[string]string
	fetcher    Fetcher
	server     *http.Server
	work       sync.WaitGroup
	closeOnce  sync.Once
	closeErr   error
}

// Open binds a fresh loopback-only origin for an exact static site. It does
// not open a browser; the Endpoint's explicit browser action remains separate.
func Open(config Config) (*Server, error) {
	return open(config, "")
}

func open(config Config, hostname string) (*Server, error) {
	if err := validate(config); err != nil {
		return nil, err
	}
	token := make([]byte, 16)
	if _, err := rand.Read(token); err != nil {
		return nil, err
	}
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return nil, err
	}
	cloned := cloneConfig(config)
	originHost, originErr := originHostFor(listener, hostname)
	if originErr != nil {
		_ = listener.Close()
		return nil, originErr
	}
	running := &Server{listener: listener, originHost: originHost, basePath: "/site/" + hex.EncodeToString(token) + "/",
		target: cloned.Target, document: cloned.Document, resources: cloned.Resources}
	running.server = &http.Server{Handler: http.HandlerFunc(running.serve), ReadHeaderTimeout: time.Second, IdleTimeout: 5 * time.Second}
	running.work.Add(1)
	go func() { defer running.work.Done(); _ = running.server.Serve(listener) }()
	return running, nil
}

// URL returns the one browser URL that represents the exact active connection.
func (server *Server) URL() string {
	if server == nil || server.listener == nil {
		return ""
	}
	return "http://" + server.originHost + server.basePath
}

// Close withdraws the local origin and joins its HTTP server.
func (server *Server) Close() error {
	if server == nil || server.server == nil {
		return nil
	}
	server.closeOnce.Do(func() {
		if closer, ok := server.fetcher.(io.Closer); ok {
			server.closeErr = errors.Join(server.closeErr, closer.Close())
		}
		server.closeErr = errors.Join(server.closeErr, server.server.Shutdown(context.Background()))
		server.work.Wait()
	})
	return server.closeErr
}

func (server *Server) serve(writer http.ResponseWriter, request *http.Request) {
	if request.Method != http.MethodGet && request.Method != http.MethodHead {
		writer.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	if request.Host != server.originHost || request.URL.IsAbs() || request.URL.RawQuery != "" || request.URL.Fragment != "" || !strings.HasPrefix(request.URL.Path, server.basePath) {
		writer.WriteHeader(http.StatusNotFound)
		return
	}
	name := strings.TrimPrefix(request.URL.Path, server.basePath)
	resource, err := server.resource(request.Context(), request.Method, name)
	if err != nil {
		if errors.Is(err, errUnknownResource) {
			writer.WriteHeader(http.StatusNotFound)
			return
		}
		writer.WriteHeader(http.StatusBadGateway)
		return
	}
	writer.Header().Set("Content-Type", resource.ContentType)
	writer.Header().Set("Content-Length", strconv.Itoa(len(resource.Body)))
	writer.Header().Set("Content-Security-Policy", contentPolicy)
	writer.Header().Set("Cache-Control", "no-store")
	writer.Header().Set("Referrer-Policy", "no-referrer")
	writer.Header().Set("X-Content-Type-Options", "nosniff")
	writer.Header().Set("Cross-Origin-Opener-Policy", "same-origin")
	writer.WriteHeader(http.StatusOK)
	if request.Method == http.MethodGet {
		_, _ = writer.Write(resource.Body)
	}
}

func (server *Server) resource(ctx context.Context, method, name string) (Resource, error) {
	if server.fetcher != nil {
		remote, found := server.routes[name]
		if !found {
			return Resource{}, errUnknownResource
		}
		fetchMethod := method
		if fetchMethod == http.MethodHead {
			fetchMethod = http.MethodGet
		}
		response, err := server.fetcher.Fetch(ctx, Request{Method: fetchMethod, Path: remote})
		if err != nil {
			return Resource{}, err
		}
		if response.ContentType == "" || len(response.Body) == 0 || len(response.Body) > 1<<20 {
			return Resource{}, errors.New("reference Service response is invalid")
		}
		return Resource(response), nil
	}
	if name == "" {
		return server.document, nil
	}
	resource, found := server.resources[name]
	if !found {
		return Resource{}, errUnknownResource
	}
	return resource, nil
}

func validate(config Config) error {
	if config.Target == [32]byte{} || !validResource(config.Document) || len(config.Resources) > 32 {
		return errors.New("reference site configuration is incomplete or outside its bound")
	}
	for name, resource := range config.Resources {
		if name == "" || name == "." || name != path.Clean(name) || strings.HasPrefix(name, "/") || strings.HasPrefix(name, "../") || strings.Contains(name, "\\") || !validResource(resource) {
			return errors.New("reference site resource is invalid")
		}
	}
	return nil
}

var errUnknownResource = errors.New("reference Site resource is not declared")

func validResource(resource Resource) bool {
	return resource.ContentType != "" && len(resource.ContentType) <= 128 && len(resource.Body) > 0 && len(resource.Body) <= 1<<20
}

func originHostFor(listener net.Listener, hostname string) (string, error) {
	originHost := listener.Addr().String()
	if hostname == "" {
		return originHost, nil
	}
	_, port, err := net.SplitHostPort(originHost)
	if err != nil {
		return "", err
	}
	return net.JoinHostPort(hostname, port), nil
}

func cloneConfig(config Config) Config {
	result := Config{Target: config.Target, Document: Resource{ContentType: config.Document.ContentType, Body: append([]byte(nil), config.Document.Body...)},
		Resources: make(map[string]Resource, len(config.Resources))}
	for name, resource := range config.Resources {
		result.Resources[name] = Resource{ContentType: resource.ContentType, Body: append([]byte(nil), resource.Body...)}
	}
	return result
}
