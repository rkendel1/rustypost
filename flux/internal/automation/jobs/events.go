package jobs

import "sync"

type EventType string

const (
	EventQueued    EventType = "queued"
	EventRunning   EventType = "running"
	EventProgress  EventType = "progress"
	EventCompleted EventType = "completed"
	EventFailed    EventType = "failed"
)

type Event struct {
	Type EventType `json:"type"`
	Job  Job       `json:"job"`
}

type Publisher struct {
	mu          sync.RWMutex
	subscribers map[chan Event]struct{}
}

func NewPublisher() *Publisher {
	return &Publisher{subscribers: map[chan Event]struct{}{}}
}

// Subscribe registers a listener and returns an unsubscribe callback.
// Consumers should stop reading when the channel closes; a channel may be
// closed automatically if the consumer falls behind and its buffer stays full.
func (p *Publisher) Subscribe(buffer int) (<-chan Event, func()) {
	ch := make(chan Event, buffer)
	p.mu.Lock()
	p.subscribers[ch] = struct{}{}
	p.mu.Unlock()
	return ch, func() {
		p.mu.Lock()
		delete(p.subscribers, ch)
		close(ch)
		p.mu.Unlock()
	}
}

func (p *Publisher) Publish(evt Event) {
	p.mu.RLock()
	slow := make([]chan Event, 0)
	for ch := range p.subscribers {
		select {
		case ch <- evt:
		default:
			slow = append(slow, ch)
		}
	}
	p.mu.RUnlock()
	if len(slow) == 0 {
		return
	}
	p.mu.Lock()
	for _, ch := range slow {
		if _, ok := p.subscribers[ch]; ok {
			delete(p.subscribers, ch)
			close(ch)
		}
	}
	p.mu.Unlock()
}
