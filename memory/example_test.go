package memory

import (
	"context"
	"fmt"
)

// ExampleNewMemoryRegistry demonstrates creating a registry backed by an
// in-memory store and performing a round-trip store/retrieve.
func ExampleNewMemoryRegistry() {
	store := NewInMemoryStore()
	reg := NewMemoryRegistry(Config{DefaultStore: store})

	// Store an entry in the default store.
	ctx := context.Background()
	err := store.Set(ctx, MemoryEntry{
		Key:   "greeting",
		Value: "hello world",
		Tier:  TierSemantic,
	})
	if err != nil {
		fmt.Println("set error:", err)
		return
	}

	// Retrieve via the default store accessor.
	entry, found, err := reg.Default().Get(ctx, "greeting")
	if err != nil || !found {
		fmt.Println("get error:", err, "found:", found)
		return
	}
	fmt.Println("key:", entry.Key)
	fmt.Println("value:", entry.Value)
	fmt.Println("tier:", entry.Tier)
	// Output:
	// key: greeting
	// value: hello world
	// tier: semantic
}
