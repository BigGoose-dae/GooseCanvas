package events

import "sync"

// Notifications are coalesced: subscribers always read a fresh database snapshot.
// Slow browsers cannot block generation workers, and reconnects need no replay log.
type Hub struct {
	mu          sync.Mutex
	subscribers map[chan struct{}]uint64
}

func New() *Hub { return &Hub{subscribers: make(map[chan struct{}]uint64)} }

func (h *Hub) Subscribe(workspace uint64) (<-chan struct{}, func()) {
	ch := make(chan struct{}, 1)
	h.mu.Lock()
	h.subscribers[ch] = workspace
	h.mu.Unlock()
	return ch, func() { h.mu.Lock(); delete(h.subscribers, ch); h.mu.Unlock() }
}

func (h *Hub) Notify(workspace uint64) {
	if h == nil {
		return
	}
	h.mu.Lock()
	defer h.mu.Unlock()
	for ch, id := range h.subscribers {
		if workspace == 0 || id == workspace {
			select {
			case ch <- struct{}{}:
			default:
			}
		}
	}
}
