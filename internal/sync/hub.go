package sync

import "sync"

type hub struct {
	mu      sync.Mutex
	clients map[chan Event]struct{}
}

func newHub() *hub {
	return &hub{clients: make(map[chan Event]struct{})}
}

func (h *hub) subscribe(buffer int) chan Event {
	ch := make(chan Event, buffer)
	h.mu.Lock()
	h.clients[ch] = struct{}{}
	h.mu.Unlock()
	return ch
}

func (h *hub) unsubscribe(ch chan Event) {
	h.mu.Lock()
	if _, ok := h.clients[ch]; ok {
		delete(h.clients, ch)
		close(ch)
	}
	h.mu.Unlock()
}

func (h *hub) publish(ev Event) {
	h.mu.Lock()
	defer h.mu.Unlock()
	for ch := range h.clients {
		select {
		case ch <- ev:
		default:
			// Slow consumer: drop rather than block the poller.
		}
	}
}
