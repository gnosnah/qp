package qp

const initIterStackSize = 256

type Iterator struct {
	tr    *Trie
	stack []*trieNode
	idx   int // stack index
}

// Iterator returns a new iterator for traversing the trie.
// The iterator starts at the root node and can be used to iterate
// through all key-value pairs stored in the trie in lexicographical order.
// Returns nil if the trie is empty.
func (tr *Trie) Iterator() *Iterator {
	var it Iterator
	it.tr = tr
	it.stack = make([]*trieNode, initIterStackSize)
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
// If there are no more items to return, ok will be false.
// The returned key and value should not be modified by the caller.
func (it *Iterator) Next() (key []byte, value any, ok bool) {
	if it.idx <= 0 {
		return nil, nil, false
	}
	if it.tr.Size() == 1 {
		leaf := it.tr.root.(*leafNode)
		it.idx = 0
		return leaf.key, leaf.value, true
	}
	return it.nextLeaf()
}

func (it *Iterator) push(n *trieNode) {
	if it.idx >= len(it.stack) {
		it.stack = append(it.stack, nil)
	}
	it.stack[it.idx] = n
	it.idx++
}

func (it *Iterator) firstLeaf() (key []byte, value any, ok bool) {
	for {
		n := it.stack[it.idx-1]
		switch (*n).(type) {
		case *branchNode:
			bn := (*n).(*branchNode)
			nextNode := bn.twig(0)
			it.push(nextNode)
		case *leafNode:
			leaf := (*n).(*leafNode)
			return leaf.key, leaf.value, true
		}
	}
}

func (it *Iterator) nextLeaf() (key []byte, value any, ok bool) {
	n := it.stack[it.idx-1]
	switch (*n).(type) {
	case *branchNode:
		return it.firstLeaf()
	case *leafNode:
		for ; it.idx >= 2; it.idx-- {
			n = it.stack[it.idx-1]
			p := it.stack[it.idx-2]
			bn := (*p).(*branchNode)
			tIdx := bn.twigIdx(*n)
			if bn.twigTail(tIdx) {
				continue
			}

			it.stack[it.idx-1] = bn.twig(tIdx + 1)
			return it.firstLeaf()
		}
	}
	it.idx = 0
	return nil, nil, false
}
