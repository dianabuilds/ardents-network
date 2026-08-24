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
	"time"
)

// Request is one normalized static-site request from the local Adapter. It
// contains neither browser headers nor an arbitrary URL.
type Request struct {
	Method, Path string
}

// Response is one bounded successful HTTP response acquired through an
// already-authenticated Service Connection.
type Response struct {
	ContentType string
	Body        []byte
}

// Fetcher obtains one declared static response from the authenticated Service
// Connection. It has no Target, route, Browser, or remote-address authority.
type Fetcher interface {
	Fetch(context.Context, Request) (Response, error)
}

// LiveConfig binds a pre-authenticated Target and a closed local-to-Service
// resource mapping. Local names are relative to the presentation root; remote
// paths are exact origin-form HTTP paths on the selected Service.
type LiveConfig struct {
	Target  [32]byte
	Routes  map[string]string
	Fetcher Fetcher
}

// OpenLive creates one fresh browser origin that can fetch only its declared
// static routes over the supplied authenticated Service Connection.
func OpenLive(config LiveConfig) (*Server, error) {
	if err := validateLive(config); err != nil {
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
	running := &Server{listener: listener, originHost: listener.Addr().String(), basePath: "/site/" + hex.EncodeToString(token) + "/",
		target: config.Target, routes: cloneRoutes(config.Routes), fetcher: config.Fetcher}
	running.server = &http.Server{Handler: http.HandlerFunc(running.serve), ReadHeaderTimeout: time.Second, IdleTimeout: 5 * time.Second}
	running.work.Add(1)
	go func() { defer running.work.Done(); _ = running.server.Serve(listener) }()
	if observed, ok := config.Fetcher.(interface{ Done() <-chan struct{} }); ok {
		go func() { <-observed.Done(); _ = running.Close() }()
	}
	return running, nil
}

func validateLive(config LiveConfig) error {
	if config.Target == [32]byte{} || config.Fetcher == nil || len(config.Routes) == 0 || len(config.Routes) > 33 {
		return errors.New("live Reference Site configuration is incomplete or outside its bound")
	}
	for name, remote := range config.Routes {
		if (name != "" && !validResourceName(name)) || !validRemotePath(remote) {
			return errors.New("live Reference Site route is invalid")
		}
	}
	if config.Routes[""] != "/" {
		return errors.New("live Reference Site lacks its declared document route")
	}
	return nil
}

func validResourceName(name string) bool {
	return name != "." && name == path.Clean(name) && !strings.HasPrefix(name, "/") &&
		!strings.HasPrefix(name, "../") && !strings.Contains(name, "\\")
}

func validRemotePath(value string) bool {
	return value != "" && strings.HasPrefix(value, "/") && value == path.Clean(value) && !strings.Contains(value, "?") &&
		!strings.Contains(value, "#") && !strings.Contains(value, "\\")
}

func cloneRoutes(routes map[string]string) map[string]string {
	result := make(map[string]string, len(routes))
	for name, remote := range routes {
		result[name] = remote
	}
	return result
}
