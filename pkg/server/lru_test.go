package server

import "testing"

func TestLRUHitsMisses(t *testing.T) {
	c := NewLRU[string](2)
	if _, ok := c.Get("a"); ok {
		t.Fatalf("expected miss")
	}
	c.Put("a", "1")
	c.Put("b", "2")
	if v, ok := c.Get("a"); !ok || v != "1" {
		t.Fatalf("expected hit a=1")
	}
	c.Put("c", "3") // evicts b (least recent)
	if _, ok := c.Get("b"); ok {
		t.Fatalf("expected b evicted")
	}
	if v, ok := c.Get("c"); !ok || v != "3" {
		t.Fatalf("expected hit c=3")
	}
	hits, misses := c.Stats()
	if hits != 2 || misses != 2 {
		t.Fatalf("hits=%d misses=%d", hits, misses)
	}
}

func TestLRUOverwrite(t *testing.T) {
	c := NewLRU[int](2)
	c.Put("k", 1)
	c.Put("k", 2)
	if v, ok := c.Get("k"); !ok || v != 2 {
		t.Fatalf("expected k=2 got %v ok=%v", v, ok)
	}
	if c.Len() != 1 {
		t.Fatalf("expected len 1 got %d", c.Len())
	}
}
