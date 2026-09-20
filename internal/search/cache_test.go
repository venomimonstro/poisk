package search

import (
	"strconv"
	"testing"
	"time"
)

func TestCacheCardinalityStaysBounded(t *testing.T) {
	c := NewCache(3)
	for i := 0; i < 100; i++ {
		c.Put("q-"+strconv.Itoa(i), Response{Query: strconv.Itoa(i)}, time.Minute)
	}
	c.mu.Lock()
	size := len(c.items)
	c.mu.Unlock()
	if size > 3 { t.Fatalf("cache size=%d", size) }
}

func TestCacheExpiresEntries(t *testing.T) {
	c := NewCache(2)
	c.Put("x", Response{Query: "x"}, time.Nanosecond)
	time.Sleep(time.Millisecond)
	if _, ok := c.Get("x"); ok { t.Fatal("expired entry returned") }
}
