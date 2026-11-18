package qp

const initIterStackSize = 256

// Iterator traverses a trie in lexicographical order. Create one with Trie.Iterator.
// An Iterator must not be copied by value after initialization; pass *Iterator
// instead. Call Trie.Iterator again to create an independent iterator.
type Iterator[V any] struct {
	tr    *Trie[V]
	stack []*trieNode[V]
	idx   int // stack index
}

// Iterator returns a new iterator for traversing the trie.
// The iterator starts at the root node and can be used to iterate
// through all key-value pairs stored in the trie in lexicographical order.
// On an empty trie the first call to Next reports ok == false.
//
// The iterator is valid only while the trie is not modified. Trie.Upsert and
// Trie.Delete on the same trie invalidate it, and so do Txn.Upsert and
// Txn.Delete for an iterator over the transaction's NewTrie. Afterwards Next
// may repeat keys, skip keys, or panic. A transaction does not modify the trie
// it was started from, so an iterator over that trie stays valid.
func (tr *Trie[V]) Iterator() *Iterator[V] {
	var it Iterator[V]
	it.tr = tr
	it.stack = make([]*trieNode[V], initIterStackSize)
	it.stack[0] = &tr.root
	if tr.size > 0 {
		it.idx = 1
	}
	if it.idx == 0 {
		return &it
	}

	return &it
}

// Next returns the next key-value pair in the iterator's sequence.
// If there are no more items to return, ok will be false and value is the
// zero value of V.
// The returned key and value should not be modified by the caller.
func (it *Iterator[V]) Next() (key []byte, value V, ok bool) {
	if it.idx <= 0 {
		var zero V
		return nil, zero, false
	}
	if it.tr.Size() == 1 {
		leaf := it.tr.root.(*leafNode[V])
		it.idx = 0
		return leaf.key, leaf.value, true
	}
	return it.nextLeaf()
}

func (it *Iterator[V]) push(n *trieNode[V]) {
	if it.idx >= len(it.stack) {
		it.stack = append(it.stack, nil)
	}
	it.stack[it.idx] = n
	it.idx++
}

func (it *Iterator[V]) firstLeaf() (key []byte, value V, ok bool) {
	for {
		n := it.stack[it.idx-1]
		switch (*n).(type) {
		case *branchNode[V]:
			bn := (*n).(*branchNode[V])
			nextNode := bn.twig(0)
			it.push(nextNode)
		case *leafNode[V]:
			leaf := (*n).(*leafNode[V])
			return leaf.key, leaf.value, true
		default:
			panic(errBadNode)
		}
	}
}

func (it *Iterator[V]) nextLeaf() (key []byte, value V, ok bool) {
	n := it.stack[it.idx-1]
	switch (*n).(type) {
	case *branchNode[V]:
		return it.firstLeaf()
	case *leafNode[V]:
		for ; it.idx >= 2; it.idx-- {
			n = it.stack[it.idx-1]
			p := it.stack[it.idx-2]
			bn := (*p).(*branchNode[V])
			tIdx := bn.twigIdx(*n)
			if bn.twigTail(tIdx) {
				continue
			}

			it.stack[it.idx-1] = bn.twig(tIdx + 1)
			return it.firstLeaf()
		}
	}
	it.idx = 0
	var zero V
	return nil, zero, false
}
