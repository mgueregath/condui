package vnc

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/coder/websocket"
)

func ValidateNoVNCURL(endpoint string) error {
	u, err := url.Parse(endpoint)
	if err != nil || u.Hostname() == "" || (u.Scheme != "ws" && u.Scheme != "wss" && u.Scheme != "http" && u.Scheme != "https") || u.User != nil || u.Fragment != "" {
		return fmt.Errorf("noVNC requires an http://, https://, ws:// or wss:// URL without credentials or fragments")
	}
	if port := u.Port(); port != "" {
		if number, err := strconv.Atoi(port); err != nil || number < 1 || number > 65535 {
			return fmt.Errorf("invalid noVNC port")
		}
	}
	if u.Scheme == "http" || u.Scheme == "https" {
		if route := u.Query().Get("path"); route != "" {
			parsed, err := url.Parse(route)
			if err != nil || parsed.IsAbs() || parsed.Host != "" || parsed.User != nil || parsed.Fragment != "" {
				return fmt.Errorf("noVNC path must be a relative WebSocket path on the same server")
			}
		}
	}
	return nil
}

// HTTP viewer URLs use a same-origin path parameter, common websockify paths,
// then the original path. Explicit WebSocket URLs are used exactly as supplied.
func webSocketCandidates(viewer *url.URL) []*url.URL {
	base := *viewer
	if base.Scheme == "ws" || base.Scheme == "wss" {
		return []*url.URL{&base}
	}
	if base.Scheme == "https" {
		base.Scheme = "wss"
	} else {
		base.Scheme = "ws"
	}
	query := base.Query()
	route := query.Get("path")
	query.Del("path")
	base.RawQuery = query.Encode()
	candidates := []*url.URL{}
	add := func(candidate *url.URL) {
		// Keep explicit endpoint parameters and any viewer token parameters.
		merged := candidate.Query()
		for key, values := range query {
			if !merged.Has(key) {
				merged[key] = values
			}
		}
		candidate.RawQuery = merged.Encode()
		for _, existing := range candidates {
			if existing.String() == candidate.String() {
				return
			}
		}
		candidates = append(candidates, candidate)
	}
	if route != "" {
		parsed, _ := url.Parse(route) // Validated by ValidateNoVNCURL.
		add(base.ResolveReference(parsed))
		return candidates
	}
	add(base.ResolveReference(&url.URL{Path: "./websockify"}))
	if !strings.HasSuffix(base.Path, "/") && base.Path != "" {
		directory := base
		directory.Path = strings.TrimSuffix(base.Path, "/") + "/websockify"
		directory.RawPath = ""
		add(&directory)
	}
	add(base.ResolveReference(&url.URL{Path: "/websockify"}))
	add(&base)
	return candidates
}

// DialWebSocket accepts binary RFB streams, as served by websockify.
func DialWebSocket(ctx context.Context, endpoint string) (net.Conn, error) {
	if err := ValidateNoVNCURL(endpoint); err != nil {
		return nil, err
	}
	viewer, _ := url.Parse(endpoint)
	fromHTTP := viewer.Scheme == "http" || viewer.Scheme == "https"
	headers := http.Header{}
	if fromHTTP {
		headers.Set("Origin", viewer.Scheme+"://"+viewer.Host)
	}
	dialCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	var status int
	for _, candidate := range webSocketCandidates(viewer) {
		attemptCtx, attemptCancel := context.WithTimeout(dialCtx, 3*time.Second)
		ws, response, err := websocket.Dial(attemptCtx, candidate.String(), &websocket.DialOptions{Subprotocols: []string{"binary"}, HTTPHeader: headers})
		attemptCancel()
		if err != nil {
			if response != nil {
				status = response.StatusCode
			}
			if !fromHTTP || dialCtx.Err() != nil || status == http.StatusUnauthorized || status == http.StatusForbidden {
				break
			}
			continue
		}
		stream := websocket.NetConn(ctx, ws, websocket.MessageBinary)
		ws.SetReadLimit(16 * 1024 * 1024)
		// Once an endpoint accepts WebSocket, report its RFB/authentication errors
		// directly instead of opening more endpoints.
		return authenticateTransport(ctx, stream, "", "")
	}
	if ctx.Err() != nil {
		return nil, fmt.Errorf("connection canceled")
	}
	if fromHTTP {
		if status == http.StatusUnauthorized || status == http.StatusForbidden {
			return nil, fmt.Errorf("noVNC HTTP server requires authorization (HTTP %d)", status)
		}
		return nil, fmt.Errorf("unable to resolve the noVNC WebSocket from the HTTP URL; use the viewer URL with ?path=... or its exact ws:// or wss:// endpoint")
	}
	if status != 0 {
		return nil, fmt.Errorf("noVNC WebSocket upgrade failed (HTTP %d); check the WebSocket path", status)
	}
	return nil, fmt.Errorf("noVNC WebSocket endpoint is unreachable or TLS verification failed")
}

func TestWebSocket(ctx context.Context, endpoint string) error {
	conn, err := DialWebSocket(ctx, endpoint)
	if err != nil {
		return err
	}
	defer conn.Close()
	return testDesktop(ctx, conn)
}
