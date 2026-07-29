package jobs

import (
	"sync"
	"time"
)

type Queue struct {
	mu    sync.RWMutex
	order []string
	items map[string]Job
	ch    chan string
}

func NewQueue(buffer int) *Queue {
	if buffer <= 0 {
		buffer = 64
	}
	return &Queue{
		items: map[string]Job{},
		ch:    make(chan string, buffer),
	}
}

func (q *Queue) Enqueue(job Job) {
	now := time.Now().UTC()
	if job.CreatedAt.IsZero() {
		job.CreatedAt = now
	}
	job.UpdatedAt = now
	job.Status = StatusQueued
	q.mu.Lock()
	q.items[job.ID] = job
	q.order = append(q.order, job.ID)
	q.mu.Unlock()
	q.ch <- job.ID
}

func (q *Queue) Next() <-chan string {
	return q.ch
}

func (q *Queue) Update(job Job) {
	job.UpdatedAt = time.Now().UTC()
	q.mu.Lock()
	q.items[job.ID] = job
	q.mu.Unlock()
}

func (q *Queue) Get(id string) (Job, bool) {
	q.mu.RLock()
	defer q.mu.RUnlock()
	j, ok := q.items[id]
	return j, ok
}

func (q *Queue) List() []Job {
	q.mu.RLock()
	defer q.mu.RUnlock()
	out := make([]Job, 0, len(q.order))
	for _, id := range q.order {
		if j, ok := q.items[id]; ok {
			out = append(out, j)
		}
	}
	return out
}
