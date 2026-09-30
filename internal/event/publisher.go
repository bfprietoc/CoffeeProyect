package event

// Publisher dispatches domain events to subscribers.
// A nil Publisher is valid and silently drops all events.
type Publisher interface {
	Publish(topic string, payload any) error
}
