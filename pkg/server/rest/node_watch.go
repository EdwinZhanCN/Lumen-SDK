package rest

import (
	"sync"

	"github.com/edwinzhancn/lumen-sdk/pkg/client"
	"github.com/edwinzhancn/lumen-sdk/pkg/discovery"
	ws "github.com/gofiber/contrib/websocket"
	"github.com/gofiber/fiber/v2"
	"go.uber.org/zap"
)

// nodeWatchHub implements the push-based discovery endpoint consumed by
// discovery.PushResolver: GET /v1/nodes/watch upgrades to a WebSocket that
// receives a "snapshot" message followed by incremental "added"/"removed"
// diffs of active nodes.
//
// One hub serves all WebSocket clients. The node watcher callback is
// registered once, and all connection writes are serialized under the hub
// mutex (WebSocket connections do not support concurrent writers).
type nodeWatchHub struct {
	client  *client.LumenClient
	logger  *zap.Logger
	handler fiber.Handler

	watchOnce sync.Once
	mu        sync.Mutex
	clients   map[*ws.Conn]struct{}
	prevNodes map[string]struct{}
}

func newNodeWatchHub(c *client.LumenClient, logger *zap.Logger) *nodeWatchHub {
	if logger == nil {
		logger = zap.NewNop()
	}
	hub := &nodeWatchHub{
		client:    c,
		logger:    logger,
		clients:   make(map[*ws.Conn]struct{}),
		prevNodes: make(map[string]struct{}),
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
		if h.client != nil {
			h.client.WatchNodes(h.broadcast)
		}
	})

	var nodes []*discovery.NodeInfo
	if h.client != nil {
		nodes = h.client.GetNodes()
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

	// Keep the connection alive; broadcasts push diffs. Any read error means
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

// broadcast diffs the active node set against the previous one and pushes
// added/removed events to every connected client.
func (h *nodeWatchHub) broadcast(nodes []*discovery.NodeInfo) {
	current := make(map[string]*discovery.NodeInfo, len(nodes))
	for _, n := range nodes {
		if n != nil && n.IsActive() {
			current[n.ID] = n
		}
	}

	h.mu.Lock()
	defer h.mu.Unlock()

	var msgs []wsNodeEvent
	for id, node := range current {
		if _, ok := h.prevNodes[id]; !ok {
			msgs = append(msgs, nodeAddedMsg(node))
		}
	}
	for id := range h.prevNodes {
		if _, ok := current[id]; !ok {
			msgs = append(msgs, nodeRemovedMsg(id))
		}
	}

	h.prevNodes = make(map[string]struct{}, len(current))
	for id := range current {
		h.prevNodes[id] = struct{}{}
	}

	if len(msgs) == 0 || len(h.clients) == 0 {
		return
	}
	for conn := range h.clients {
		for _, msg := range msgs {
			if err := conn.WriteJSON(msg); err != nil {
				h.logger.Debug("node watch: event write failed", zap.Error(err))
				break
			}
		}
	}
}

// ---- wire format (must stay compatible with discovery.PushResolver) ----

type wsNodeInfo struct {
	NodeID  string            `json:"node_id"`
	Address string            `json:"address"`
	Tasks   []string          `json:"tasks,omitempty"`
	Txt     map[string]string `json:"txt,omitempty"`
}

type wsNodeEvent struct {
	Type   string       `json:"type"` // "snapshot", "added", "removed"
	Nodes  []wsNodeInfo `json:"nodes,omitempty"`
	Node   *wsNodeInfo  `json:"node,omitempty"`
	NodeID string       `json:"node_id,omitempty"`
}

func wsNodeInfoFrom(n *discovery.NodeInfo) wsNodeInfo {
	tasks := make([]string, 0, len(n.Tasks))
	for _, t := range n.Tasks {
		if t != nil && t.Name != "" {
			tasks = append(tasks, t.Name)
		}
	}
	var txt map[string]string
	if n.Version != "" || n.Runtime != "" {
		txt = make(map[string]string, 2)
		if n.Version != "" {
			txt["v"] = n.Version
		}
		if n.Runtime != "" {
			txt["runtime"] = n.Runtime
		}
	}
	return wsNodeInfo{
		NodeID:  n.ID,
		Address: n.Address,
		Tasks:   tasks,
		Txt:     txt,
	}
}

func nodeSnapshotMsg(nodes []*discovery.NodeInfo) wsNodeEvent {
	infos := make([]wsNodeInfo, 0, len(nodes))
	for _, n := range nodes {
		if n == nil || !n.IsActive() {
			continue
		}
		infos = append(infos, wsNodeInfoFrom(n))
	}
	return wsNodeEvent{Type: "snapshot", Nodes: infos}
}

func nodeAddedMsg(n *discovery.NodeInfo) wsNodeEvent {
	info := wsNodeInfoFrom(n)
	return wsNodeEvent{Type: "added", Node: &info}
}

func nodeRemovedMsg(nodeID string) wsNodeEvent {
	return wsNodeEvent{Type: "removed", NodeID: nodeID}
}
