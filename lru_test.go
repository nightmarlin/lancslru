// tests can be in the same package (package lru) or a special test package
// (lru_test). Use of the latter lets you treat the package under test as a
// black box - so you're not accidentally testing implementation details.
package lancslru_test

import (
	"context"
	"fmt"
	"runtime"
	"testing"
	"time"

	"github.com/nightmarlin/lancslru"
)

// A test is just a function named "Test..." that takes a *testing.T, in a file
// named "*_test.go".
func TestCache_Lookup(t *testing.T) {
	t.Parallel() // tests aren't parallel by default, but it's trivial to make them so.

	// tests can have nested tests to group related tests - which themselves can be run in parallel.
	t.Run(
		"lookup of same key is cached",
		func(t *testing.T) {
			t.Parallel()

			var (
				k, v = "key", 5

				ctx = context.Background() // todo(go1.24): use t.Context()
				c   = lancslru.New[string, int](1)
			)

			res, err := c.Lookup(
				ctx,
				k,
				// good design allows you to pass functionality around instead of testing an entire program.
				func(_ context.Context, got string) (int, error) {
					if got != k {
						t.Errorf("lookup func: got key %q, want %q", got, k)
					}
					return v, nil
				},
			)
			if err != nil {
				// tests are allowed to continue even if some assertions failed
				t.Errorf("lookup: got error %v, want nil", err)
			} else if res != v {
				t.Errorf("lookup: got value %v, want %v", res, v)
			}

			res, err = c.Lookup(
				ctx,
				k,
				func(context.Context, string) (int, error) {
					t.Errorf("lookup func called, but the value should have been cached")
					return v, nil
				},
			)
			if err != nil {
				t.Errorf("lookup: got error %v, want nil", err)
			} else if res != v {
				t.Errorf("lookup: got value %v, want %v", res, v)
			}
		},
	)

	t.Run(
		"returns error if lookup func returns error",
		func(t *testing.T) {
			t.Parallel()

			var (
				k, expectErr = "key", fmt.Errorf("broken")

				ctx = context.Background() // todo(go1.24): use t.Context(). Go is always evolving!
				c   = lancslru.New[string, int](1)
			)

			res, err := c.Lookup(
				ctx,
				k,
				func(context.Context, string) (int, error) { return 0, expectErr },
			)
			if err == nil {
				t.Errorf("lookup: got nil error, want %v", expectErr)
			}
			if res != 0 {
				t.Errorf("lookup: got value %v, want 0", res)
			}
		},
	)

	t.Run(
		"evicts least-recently-used entry",
		func(t *testing.T) {
			t.Parallel()

			var (
				kv   = map[int]string{1: "k1", 2: "k2", 3: "k3"}
				load = func(_ context.Context, key int) (string, error) { return kv[key], nil }

				ctx = context.Background() // todo(go1.24): use t.Context()
				c   = lancslru.New[int, string](2)
			)

			_, _ = c.Lookup(ctx, 1, load) // lru: k1
			_, _ = c.Lookup(ctx, 2, load) // lru: k1
			_, _ = c.Lookup(ctx, 1, load) // lru: k2
			_, _ = c.Lookup(ctx, 3, load) // should evict k2

			// lru.Cache.cleanup is run in a goroutine - so put some work in to trigger it
			for range 3 {
				time.Sleep(time.Millisecond)
				runtime.Gosched()
			}

			var didLookup bool
			_, _ = c.Lookup(
				ctx,
				2,
				func(_ context.Context, key int) (string, error) {
					didLookup = true
					return kv[key], nil
				},
			)
			if !didLookup {
				t.Errorf("lookup func should have been called for key `3`, as it should have been evicted")
			}
		},
	)

}
