package hostbroker

import (
	"sync"
	"time"

	"github.com/edwinzhancn/lumen-sdk/pkg/discovery"
	ws "github.com/gofiber/contrib/websocket"
	"github.com/gofiber/fiber/v2"
	"go.uber.org/zap"
)

// nodeWatchHub implements the push-based discovery endpoint consumed by
// discovery.BrokerResolver: GET /v1/nodes/watch upgrades to a WebSocket that
// receives an initial snapshot and a replacement snapshot whenever the
// catalog changes.
//
// One hub serves all WebSocket clients. The node watcher callback is
// registered once, and all connection writes are serialized under the hub
// mutex (WebSocket connections do not support concurrent writers).
type nodeWatchHub struct {
	catalog NodeCatalog
	logger  *zap.Logger
	handler fiber.Handler

	watchOnce sync.Once
	mu        sync.Mutex
	clients   map[*ws.Conn]struct{}
}

func newNodeWatchHub(catalog NodeCatalog, logger *zap.Logger) *nodeWatchHub {
	if logger == nil {
		logger = zap.NewNop()
	}
	hub := &nodeWatchHub{
		catalog: catalog,
		logger:  logger,
		clients: make(map[*ws.Conn]struct{}),
	}
	hub.handler = ws.New(hub.serve)
	return hub
}

func (h *nodeWatchHub) upgrade(c *fiber.Ctx) error {
	if !ws.IsWebSocketUpgrade(c) {
		return fiber.ErrUpgradeRequired
	}
	return h.handler(c)
}

func (h *nodeWatchHub) serve(conn *ws.Conn) {
	h.watchOnce.Do(func() {
		if h.catalog != nil {
			h.catalog.WatchNodes(h.broadcast)
		}
	})

	var nodes []*discovery.NodeInfo
	if h.catalog != nil {
		nodes = h.catalog.GetNodes()
	}

	h.mu.Lock()
	h.clients[conn] = struct{}{}
	// Send the snapshot under the hub lock so it cannot interleave with a
	// concurrent broadcast write on the same connection.
	err := conn.WriteJSON(nodeSnapshotMsg(nodes))
	h.mu.Unlock()
	if err != nil {
		h.logger.Debug("node watch: snapshot write failed", zap.Error(err))
		h.dropClient(conn)
		return
	}

	// Keep the connection alive; broadcasts push replacement snapshots. Any read error means
	// the client went away.
	defer h.dropClient(conn)
	for {
		if _, _, err := conn.ReadMessage(); err != nil {
			return
		}
	}
}

func (h *nodeWatchHub) dropClient(conn *ws.Conn) {
	h.mu.Lock()
	delete(h.clients, conn)
	h.mu.Unlock()
}

// Close closes every currently connected watch client. fasthttp makes a
// hijacked connection's own Close a no-op by design (KeepHijackedConns
// defaults to false — fasthttp closes the real connection itself once the
// handler returns), so this uses SetReadDeadline instead: that reliably
// unblocks serve's ReadMessage loop with an i/o timeout, serve returns, and
// fasthttp performs the real close once the handler has returned. See
// Host Broker shutdown closes hijacked connections explicitly so that watch
// goroutines exit promptly.
//
// The lock is held for the whole call (matching broadcast) so a
// connection's own teardown can't race this same object concurrently with
// gofiber-contrib's *ws.Conn recycling into its package-level sync.Pool.
func (h *nodeWatchHub) Close() {
	h.mu.Lock()
	defer h.mu.Unlock()
	for conn := range h.clients {
		_ = conn.SetReadDeadline(time.Now())
	}
}

// broadcast sends one complete address snapshot. Full snapshots avoid a second
// node-state machine inside the Broker and correctly propagate endpoint changes
// for an existing node identity.
func (h *nodeWatchHub) broadcast(nodes []*discovery.NodeInfo) {
	msg := nodeSnapshotMsg(nodes)

	h.mu.Lock()
	defer h.mu.Unlock()
	for conn := range h.clients {
		if err := conn.WriteJSON(msg); err != nil {
			h.logger.Debug("node watch: snapshot write failed", zap.Error(err))
			delete(h.clients, conn)
			_ = conn.SetReadDeadline(time.Now())
		}
	}
}
