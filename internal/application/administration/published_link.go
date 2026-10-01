package administration

import (
	"context"
	"net"
)

// PublishedLinkProvider exposes the current committed publication through the
// same separately authorized local owner. A Link is a destination, not proof
// of future availability or permission to publish another Service.
type PublishedLinkProvider interface {
	PublishedLink(context.Context) (string, error)
}

const maximumPublishedLink = 512

func (server *server) handlePublishedLink(connection *net.UnixConn) {
	owner, ok := server.owner.(PublishedLinkProvider)
	if !ok {
		writeResponse(connection, "unavailable\n")
		return
	}
	link, err := owner.PublishedLink(server.ctx)
	if err != nil || !validPublishedLink(link) {
		writeResponse(connection, "unavailable\n")
		return
	}
	raw := append([]byte("link\n"), byte(len(link)>>8), byte(len(link)))
	raw = append(raw, link...)
	writeResponse(connection, string(raw))
}

// The adapter transports only bounded printable destination text. Canonical
// Target/Network validation remains in Endpoint, not this local transport.
func validPublishedLink(link string) bool {
	if len(link) == 0 || len(link) > maximumPublishedLink {
		return false
	}
	for _, value := range []byte(link) {
		if value < 0x21 || value > 0x7e {
			return false
		}
	}
	return true
}
