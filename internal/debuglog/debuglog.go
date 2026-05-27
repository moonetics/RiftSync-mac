package debuglog

import (
	"sync"
	"time"
)

type Event struct {
	TS      float64 `json:"ts"`
	Kind    string  `json:"kind"`
	Message string  `json:"message"`
}

type Ring struct {
	mu     sync.Mutex
	limit  int
	events []Event
}

func New(limit int) *Ring {
	if limit <= 0 {
		limit = 100
	}
	return &Ring{
		limit:  limit,
		events: make([]Event, 0, limit),
	}
}

func (r *Ring) Add(kind, message string) {
	r.mu.Lock()
	defer r.mu.Unlock()

	if len(r.events) == r.limit {
		copy(r.events, r.events[1:])
		r.events[len(r.events)-1] = Event{}
		r.events = r.events[:len(r.events)-1]
	}

	r.events = append(r.events, Event{
		TS:      float64(time.Now().UnixMilli()) / 1000,
		Kind:    kind,
		Message: message,
	})
}

func (r *Ring) Events() []Event {
	r.mu.Lock()
	defer r.mu.Unlock()

	result := make([]Event, len(r.events))
	copy(result, r.events)
	return result
}
