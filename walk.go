package qp

type WalkFn[V any] = func(key []byte, val V) (add bool)

func defaultWalkFn[V any](key []byte, val V) (add bool) {
	return true
}

// KVPair is a key-value pair.
type KVPair[V any] struct {
	Key   []byte
	Value V
}

// Walk traverses the entire trie and applies the given function to each element's key
// and value. If the function returns true, the corresponding key-value pair is included in the result.
// The function must not modify the trie.
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
