package lancslru

import (
	"context"
	"fmt"
	"sync"
)

// A Cache uses a doubly-linked list and a lookup map into that list to
// implement a least-recently-used cache.
//
// https://en.wikipedia.org/wiki/Cache_replacement_policies#Simple_recency-based_policies
//
// A Cache is safe for concurrent use.
//
//	this is a godoc comment - it will be rendered using MD-like syntax in
//	pkg.go.dev once published, similar to javadoc (but better).
type Cache[K comparable, V any] struct {
	// ^ generics

	mux      sync.Mutex                 // built-in atomics
	cap      int                        // encapsulation
	lookup   map[K]*lruCacheEntry[K, V] // built-in hashmaps
	oldest   *lruCacheEntry[K, V]
	youngest *lruCacheEntry[K, V]
}

type lruCacheEntry[K comparable, V any] struct {
	k       K
	v       V
	younger *lruCacheEntry[K, V]
	// older   *lruCacheEntry[K, V] // todo: hoisting
}

// New creates and returns a new Cache with capacity `cap`. Once `cap` is
// reached, the least-recently accessed elements in the cache are evicted until
// the Cache contains `cap` elements.
func New[K comparable, V any](cap int) *Cache[K, V] {
	return &Cache[K, V]{
		cap: cap,
		// make it possible to hold more than `cap` entries to reduce likelihood of
		// unnecessary map growth
		lookup: make(map[K]*lruCacheEntry[K, V], cap+2),
	}
}

// Lookup accesses the Cache and, if an entry is present, returns it and marks
// it as the most recently used entry. If an entry is not present, the `load`
// function is called and its returned value is added to the cache.
func (c *Cache[K, V]) Lookup(
	ctx context.Context,
	key K,
	load func(context.Context, K) (V, error),
) (V, error) {
	// this function demonstrates
	// - pointer receivers (methods attached to a type)
	// - pass-by-value semantics
	// - contexts for request-scoped operations
	// - first-class functions
	// - multiple-return values (errors as values, not as exceptions)

	defer c.mux.Unlock() // defer statements (run at end of stack frame)
	c.mux.Lock()

	if entry, ok := c.lookup[key]; ok { // compound if statements
		// todo: hoist retrieved entry to `c.youngest`
		return entry.v, nil
	}

	value, err := load(ctx, key)
	if err != nil {
		// all values are initialized to the zero value for that type
		var zero V
		return zero, fmt.Errorf("loading value for key: %w", err)
	}

	newest := &lruCacheEntry[K, V]{k: key, v: value}
	if c.youngest != nil {
		c.youngest.younger = newest
		// todo: mark original younger as older than newest entry
	}
	c.youngest = newest
	c.lookup[key] = newest

	// new entry written - evict old entries in the background on the way out
	go c.cleanup() // goroutines provide built-in lightweight concurrency
	return value, nil
}

func (c *Cache[K, V]) cleanup() {
	defer c.mux.Unlock()
	c.mux.Lock()

	for len(c.lookup) > c.cap && c.oldest != nil {
		o := c.oldest
		c.oldest = o.younger
		delete(c.lookup, o.k)
		// garbage collection - once nothing references o, it will be freed in the background
	}
}
