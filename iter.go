package qp

const initIterStackSize = 256

type iterFrame struct {
	n    trieNode
	twig int
}

// Iterator walks a trie in lexicographical order.
// It is not safe for concurrent use, and the trie must not be mutated
// while iteration is in progress.
type Iterator struct {
	stack   []iterFrame
	started bool
}

// Iterator returns a new iterator for traversing the trie
// in lexicographical order. An empty trie yields an iterator
// whose first Next call returns ok=false.
func (tr *Trie) Iterator() *Iterator {
	it := &Iterator{
		stack: make([]iterFrame, 0, initIterStackSize),
	}
	if tr.root != nil && tr.size > 0 {
		it.stack = append(it.stack, iterFrame{n: tr.root})
	}
	return it
}

// Next returns the next key-value pair in the iterator's sequence.
// If there are no more items to return, ok will be false.
// The returned key and value should not be modified by the caller.
func (it *Iterator) Next() (key []byte, value any, ok bool) {
	if len(it.stack) == 0 {
		return nil, nil, false
	}
	if !it.started {
		it.started = true
		return it.firstLeaf()
	}
	return it.advance()
}

func (it *Iterator) firstLeaf() (key []byte, value any, ok bool) {
	for {
		switch n := it.stack[len(it.stack)-1].n.(type) {
		case *leafNode:
			return n.key, n.value, true
		case *branchNode:
			it.stack[len(it.stack)-1].twig = 0
			it.stack = append(it.stack, iterFrame{n: n.twigs[0]})
		}
	}
}

func (it *Iterator) advance() (key []byte, value any, ok bool) {
	for len(it.stack) > 0 {
		top := &it.stack[len(it.stack)-1]
		bn, isBranch := top.n.(*branchNode)
		if !isBranch {
			it.stack = it.stack[:len(it.stack)-1]
			continue
		}
		top.twig++
		if top.twig < bn.twigOffsetMax() {
			it.stack = append(it.stack, iterFrame{n: bn.twigs[top.twig]})
			return it.firstLeaf()
		}
		it.stack = it.stack[:len(it.stack)-1]
	}
	return nil, nil, false
}
