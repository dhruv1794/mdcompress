package server

import (
	"container/list"
	"sync"
)

// LRU is a tiny thread-safe LRU cache with hit/miss counters.
type LRU[V any] struct {
	mu     sync.Mutex
	cap    int
	list   *list.List
	items  map[string]*list.Element
	hits   int
	misses int
}

type lruEntry[V any] struct {
	key   string
	value V
}

func NewLRU[V any](capacity int) *LRU[V] {
	if capacity <= 0 {
		capacity = 1
	}
	return &LRU[V]{
		cap:   capacity,
		list:  list.New(),
		items: make(map[string]*list.Element, capacity),
	}
}

func (l *LRU[V]) Get(key string) (V, bool) {
	l.mu.Lock()
	defer l.mu.Unlock()
	var zero V
	el, ok := l.items[key]
	if !ok {
		l.misses++
		return zero, false
	}
	l.list.MoveToFront(el)
	l.hits++
	return el.Value.(*lruEntry[V]).value, true
}

func (l *LRU[V]) Put(key string, val V) {
	l.mu.Lock()
	defer l.mu.Unlock()
	if el, ok := l.items[key]; ok {
		el.Value.(*lruEntry[V]).value = val
		l.list.MoveToFront(el)
		return
	}
	el := l.list.PushFront(&lruEntry[V]{key: key, value: val})
	l.items[key] = el
	if l.list.Len() > l.cap {
		oldest := l.list.Back()
		if oldest != nil {
			entry := oldest.Value.(*lruEntry[V])
			delete(l.items, entry.key)
			l.list.Remove(oldest)
		}
	}
}

// Stats returns cumulative hits and misses since the cache was created.
func (l *LRU[V]) Stats() (hits, misses int) {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.hits, l.misses
}

// Len returns the current number of entries.
func (l *LRU[V]) Len() int {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.list.Len()
}
