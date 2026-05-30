package event

import "sync"

// Handler is a function that processes an event payload.
type Handler func(topic string, payload any)

// Bus is a synchronous in-memory event bus.
// Publish calls all registered handlers before returning.
// Safe for concurrent use.
type Bus struct {
	mu       sync.RWMutex
	handlers map[string][]Handler
}

func NewBus() *Bus {
	return &Bus{handlers: make(map[string][]Handler)}
}

func (b *Bus) Subscribe(topic string, h Handler) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.handlers[topic] = append(b.handlers[topic], h)
}

func (b *Bus) Publish(topic string, payload any) error {
	b.mu.RLock()
	hs := b.handlers[topic]
	b.mu.RUnlock()
	for _, h := range hs {
		h(topic, payload)
	}
	return nil
}
