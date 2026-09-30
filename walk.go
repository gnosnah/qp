package qp

import "bytes"

type WalkFn = func(key []byte, val any) (add bool)

var defaultWalkFn = func(key []byte, val any) (add bool) {
	return true
}

// KVPair is a key-value pair.
type KVPair struct {
	Key   []byte
	Value any
}

// Walk traverses the trie in lexicographical order and applies f to each pair.
// If f returns true, the pair is copied into the result.
// At most max pairs are returned; max <= 0 yields a nil result.
func (tr *Trie) Walk(max int, f WalkFn) (pairs []KVPair) {
	if max <= 0 {
		return nil
	}
	if f == nil {
		f = defaultWalkFn
	}
	it := tr.Iterator()
	for len(pairs) < max {
		k, v, ok := it.Next()
		if !ok {
			break
		}
		if add := f(k, v); add {
			pairs = append(pairs, KVPair{Key: bytes.Clone(k), Value: v})
		}
	}
	return
}
