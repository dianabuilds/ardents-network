package reference

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"net"
	"net/http"
	"path"
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
	config     Config
	server     *http.Server
	work       sync.WaitGroup
}

// Open binds a fresh loopback-only origin for an exact static site. It does
// not open a browser; the Endpoint's explicit browser action remains separate.
func Open(config Config) (*Server, error) {
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
	running := &Server{listener: listener, originHost: listener.Addr().String(), basePath: "/site/" + hex.EncodeToString(token) + "/", config: cloneConfig(config)}
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
	err := server.server.Shutdown(context.Background())
	server.work.Wait()
	return err
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
	var resource Resource
	switch name {
	case "":
		resource = server.config.Document
	default:
		var found bool
		resource, found = server.config.Resources[name]
		if !found {
			writer.WriteHeader(http.StatusNotFound)
			return
		}
	}
	writer.Header().Set("Content-Type", resource.ContentType)
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

func validResource(resource Resource) bool {
	return resource.ContentType != "" && len(resource.ContentType) <= 128 && len(resource.Body) > 0 && len(resource.Body) <= 1<<20
}

func cloneConfig(config Config) Config {
	result := Config{Target: config.Target, Document: Resource{ContentType: config.Document.ContentType, Body: append([]byte(nil), config.Document.Body...)},
		Resources: make(map[string]Resource, len(config.Resources))}
	for name, resource := range config.Resources {
		result.Resources[name] = Resource{ContentType: resource.ContentType, Body: append([]byte(nil), resource.Body...)}
	}
	return result
}
