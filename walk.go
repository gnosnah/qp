package qp

type WalkFn = func(key []byte, val any) (add bool)

var defaultWalkFn = func(key []byte, val any) (add bool) {
	return true
}

// KVPair is a key-value pair.
type KVPair struct {
	Key   []byte
	Value any
}

func (tr *Trie) Walk(max int, f WalkFn) (pairs []KVPair) {
	if f == nil {
		f = defaultWalkFn
	}
	it := tr.Iterator()
	for {
		if len(pairs) >= max {
			break
		}
		k, v, ok := it.Next()
		if !ok {
			break
		}
		if add := f(k, v); add {
			pairs = append(pairs, KVPair{Key: k, Value: v})
		}
	}
	return
}
