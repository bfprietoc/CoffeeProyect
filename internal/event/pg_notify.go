package event

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"sync"

	"github.com/lib/pq"
)

// PGPublisher sends events via PostgreSQL NOTIFY.
type PGPublisher struct {
	db *sql.DB
}

func NewPGPublisher(db *sql.DB) *PGPublisher {
	return &PGPublisher{db: db}
}

func (p *PGPublisher) Publish(topic string, payload any) error {
	data, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("marshal event %s: %w", topic, err)
	}
	_, err = p.db.Exec(`SELECT pg_notify($1, $2)`, topic, string(data))
	return err
}

// PGListener listens on PostgreSQL NOTIFY channels and dispatches to handlers.
type PGListener struct {
	listener *pq.Listener
	handlers map[string][]Handler
	mu       sync.RWMutex // declared below
}

func NewPGListener(dsn string) *PGListener {
	l := pq.NewListener(dsn, 10e9, 60e9, func(ev pq.ListenerEventType, err error) {
		if err != nil {
			log.Printf("pg_notify listener: %v", err)
		}
	})
	return &PGListener{
		listener: l,
		handlers: make(map[string][]Handler),
	}
}

func (l *PGListener) Subscribe(topic string, h Handler) {
	l.mu.Lock()
	l.handlers[topic] = append(l.handlers[topic], h)
	l.mu.Unlock()
	_ = l.listener.Listen(topic)
}

// Start begins consuming notifications in a goroutine. Blocks on ctx.Done.
func (l *PGListener) Start(ctx context.Context) {
	go func() {
		for {
			select {
			case <-ctx.Done():
				l.listener.Close()
				return
			case n := <-l.listener.Notify:
				if n == nil {
					continue // reconnect signal
				}
				l.dispatch(n.Channel, n.Extra)
			}
		}
	}()
}

func (l *PGListener) dispatch(topic, raw string) {
	var payload any
	_ = json.Unmarshal([]byte(raw), &payload)

	l.mu.RLock()
	hs := l.handlers[topic]
	l.mu.RUnlock()
	for _, h := range hs {
		h(topic, payload)
	}
}
