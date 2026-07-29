package events

import (
	"sync"
	"time"
)

// Event is the generic envelope used across the automation runtime.
type Event struct {
	Name          string         `json:"name"`
	CorrelationID string         `json:"correlationId,omitempty"`
	ProjectID     string         `json:"projectId,omitempty"`
	SourceID      string         `json:"sourceId,omitempty"`
	JobID         string         `json:"jobId,omitempty"`
	PipelineID    string         `json:"pipelineId,omitempty"`
	Timestamp     time.Time      `json:"timestamp"`
	Payload       map[string]any `json:"payload,omitempty"`
}

type subscriber chan Event

// Bus is an in-process pub/sub bus keyed by topic.
type Bus struct {
	mu     sync.RWMutex
	topics map[string]map[subscriber]struct{}
}

func NewBus() *Bus {
	return &Bus{topics: map[string]map[subscriber]struct{}{}}
}

func (b *Bus) Subscribe(topic string, buffer int) (<-chan Event, func()) {
	if buffer <= 0 {
		buffer = 16
	}
	ch := make(subscriber, buffer)
	b.mu.Lock()
	if b.topics[topic] == nil {
		b.topics[topic] = map[subscriber]struct{}{}
	}
	b.topics[topic][ch] = struct{}{}
	b.mu.Unlock()

	return ch, func() {
		b.mu.Lock()
		if subs, ok := b.topics[topic]; ok {
			if _, exists := subs[ch]; exists {
				delete(subs, ch)
				close(ch)
			}
			if len(subs) == 0 {
				delete(b.topics, topic)
			}
		}
		b.mu.Unlock()
	}
}

func (b *Bus) Publish(topic string, evt Event) {
	if evt.Timestamp.IsZero() {
		evt.Timestamp = time.Now().UTC()
	}

	b.mu.RLock()
	subs := b.topics[topic]
	fullBufferSubscribers := make([]subscriber, 0, len(subs))
	for ch := range subs {
		select {
		case ch <- evt:
		default:
			fullBufferSubscribers = append(fullBufferSubscribers, ch)
		}
	}
	b.mu.RUnlock()
	if len(fullBufferSubscribers) == 0 {
		return
	}

	b.mu.Lock()
	defer b.mu.Unlock()
	for _, ch := range fullBufferSubscribers {
		if subs, ok := b.topics[topic]; ok {
			if _, exists := subs[ch]; exists {
				delete(subs, ch)
				close(ch)
			}
		}
	}
}
