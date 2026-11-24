package qp

// WalkFn decides whether a visited key-value pair is included in Walk's result.
// It must not modify the trie. A nil WalkFn accepts every pair.
type WalkFn[V any] = func(key []byte, val V) (add bool)

func defaultWalkFn[V any](key []byte, val V) (add bool) {
	return true
}

// KVPair is a key-value pair returned by Walk.
// Key belongs to the trie and must not be modified.
type KVPair[V any] struct {
	Key   []byte
	Value V
}

// Walk visits keys in lexicographical order and returns up to max pairs for
// which f returns true. A nil f accepts every pair. f must not modify the trie.
func (tr *Trie[V]) Walk(max int, f WalkFn[V]) (pairs []KVPair[V]) {
	if f == nil {
		f = defaultWalkFn[V]
	}
	it := tr.Iterator()
	for len(pairs) < max {
		k, v, ok := it.Next()
		if !ok {
			break
		}
		if add := f(k, v); add {
			pairs = append(pairs, KVPair[V]{Key: k, Value: v})
		}
	}
	return
}
