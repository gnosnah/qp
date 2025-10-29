package qp

import (
	"bytes"
	"fmt"
	"math"
)

type bitmapT = uint32      // bitmap type, 17 bits, first bit NO_BYTE
type nibbleIndexT = uint16 // nibble index type

// OnInsertValFn is a function type that processes a new value before insertion.
// It takes the new value as input and returns the final value to be used.
type OnInsertValFn = func(newVal any) (finalVal any)

// OnUpdateValFn is a function type that takes two parameters of type any (newVal and oldVal)
// and returns a finalVal of type any. It is used to handle value updates by comparing
// the new and old values and determining the final value to be used.
type OnUpdateValFn = func(newVal, oldVal any) (finalVal any)

const (
	nibbleIndexMax = math.MaxUint16
	maxKeyBytes    = nibbleIndexMax >> 1
)

var (
	errKeyEmpty   = fmt.Errorf("empty key")
	errKeyTooLong = fmt.Errorf("max key length is %d bytes", maxKeyBytes)
	errInternal   = fmt.Errorf("internal error")
)

var (
	// default newVal is finalVal
	defaultOnInsert = func(newVal any) any { return newVal }
	defaultOnUpdate = func(newVal, oldVal any) any { return newVal }
)

type Option func(*Trie)

func WithOnInsert(f OnInsertValFn) Option {
	return func(tr *Trie) {
		tr.onInsert = f
	}
}

func WithOnUpdate(f OnUpdateValFn) Option {
	return func(tr *Trie) {
		tr.onUpdate = f
	}
}

type Trie struct {
	root     trieNode
	size     int
	onInsert OnInsertValFn
	onUpdate OnUpdateValFn
}

// New creates and initializes a new Trie with the given options.
// If no onInsert or onUpdate handlers are provided, default handlers will be used.
// default onInsert: func(newVal any) any { return newVal }
// default onUpdate: func(newVal, oldVal any) any { return newVal }
func New(opts ...Option) *Trie {
	var tr Trie
	for _, opt := range opts {
		opt(&tr)
	}
	if tr.onInsert == nil {
		tr.onInsert = defaultOnInsert
	}
	if tr.onUpdate == nil {
		tr.onUpdate = defaultOnUpdate
	}
	return &tr
}

// Size returns the total number of key-value pairs stored in the trie.
func (tr *Trie) Size() int {
	return tr.size
}

func (tr *Trie) findMatch(key []byte, exactMatch bool) *leafNode {
	if tr.root == nil {
		return nil
	}
	var bn *branchNode
	ptr := &tr.root
	for {
		switch n := (*ptr).(type) {
		case *leafNode:
			return n
		case *branchNode:
			bn = n
			i := 0
			b := bn.twigBit(key)
			if bn.hasTwig(b) {
				i = bn.twigOffset(b)
			} else {
				if exactMatch {
					return nil
				}
			}
			ptr = bn.twig(i)
		}
	}
}

func (tr *Trie) findInsert(key []byte, index nibbleIndexT) (ptr *trieNode, grow bool) {
	ptr = &tr.root
	for {
		switch n := (*ptr).(type) {
		case *leafNode:
			return ptr, false
		case *branchNode:
			if index == n.index {
				return ptr, true
			}
			if index < n.index {
				return ptr, false
			}

			b := n.twigBit(key)
			if !n.hasTwig(b) {
				panic(errInternal)
			}
			i := n.twigOffset(b)
			ptr = n.twig(i)
		}
	}
}

func (tr *Trie) findDelete(key []byte) (parentBranch *trieNode, leaf *leafNode, b bitmapT) {
	if tr.root == nil {
		return nil, nil, 0
	}

	ptr := &tr.root
	for {
		switch n := (*ptr).(type) {
		case *leafNode:
			return parentBranch, n, b
		case *branchNode:
			b = n.twigBit(key)
			if !n.hasTwig(b) {
				return nil, nil, 0
			}
			i := n.twigOffset(b)
			parentBranch = ptr
			ptr = n.twig(i)
		}
	}
}

// Get retrieves the value associated with the given key from the trie.
// It returns the value and a boolean indicating whether the key was found.
// If the key is not present in the trie, it returns nil and false.
func (tr *Trie) Get(key []byte) (val any, found bool) {
	must(key)
	leaf := tr.findMatch(key, true)
	if leaf != nil && bytes.Equal(key, leaf.key) {
		return leaf.value, true
	}
	return nil, false
}

// Upsert inserts or updates a key-value pair in the trie.
// If the key already exists, it updates the value and returns the old value with isUpdate=true.
// If the key does not exist, it inserts the new key-value pair and returns nil with isUpdate=false.
func (tr *Trie) Upsert(key []byte, value any) (oldVal any, isUpdate bool) {
	must(key)

	if tr.root == nil {
		tr.root = &leafNode{key: key, value: tr.onInsert(value)}
		tr.size++
		return nil, false
	}

	leaf := tr.findMatch(key, false)
	index, match := nibbleIndex(key, leaf.key)
	if match {
		preValue := leaf.value
		leaf.value = tr.onUpdate(value, preValue)
		return preValue, true
	}

	newLeaf := &leafNode{key: key, value: tr.onInsert(value)}
	ptr, grow := tr.findInsert(key, index)
	if grow {
		bn := (*ptr).(*branchNode)
		bn.growTwigs(index, key, newLeaf)
	} else {
		bn := newBranchNode(*ptr, index, leaf.key, key, newLeaf)
		*ptr = bn
	}

	tr.size++
	return nil, false
}

// Delete removes the entry for the given key from the trie.
// It returns the value that was associated with the key and a boolean indicating
// whether the key was present in the trie.
// If the key is not found, it returns nil and false.
func (tr *Trie) Delete(key []byte) (oldVal any, found bool) {
	must(key)

	parentBn, leaf, b := tr.findDelete(key)
	if leaf == nil || !bytes.Equal(key, leaf.key) {
		return nil, false
	}
	tr.size--
	if parentBn == nil {
		// only when root is leafNode
		tr.root = nil
		return leaf.value, true
	}

	bn := (*parentBn).(*branchNode)
	if bn.twigOffsetMax() == 2 {
		other := 0
		if bn.twigOffset(b) == 0 {
			other = 1
		}
		// Move the other twig to the parent branch.
		otherTwig := bn.twig(other)
		*parentBn = *otherTwig
		return leaf.value, true
	}

	bn.removeTwig(b)
	return leaf.value, true
}

func (tr *Trie) findPrev(index nibbleIndexT, key []byte) (prev *trieNode, cur *trieNode, needCheckCur bool) {
	cur = &tr.root
	for {
		switch n := (*cur).(type) {
		case *leafNode:
			needCheckCur = true
			return
		case *branchNode:
			bn := (*cur).(*branchNode)
			if index < bn.index {
				needCheckCur = true
				return
			}
			b := bn.twigBit(key)
			i := bn.twigOffset(b)
			if i > 0 {
				prev = bn.twig(i - 1)
			}
			if index == bn.index {
				return
			}
			cur = n.twig(i)
		}
	}
}

func (tr *Trie) lastLeaf(node *trieNode) *leafNode {
	ptr := node
	for {
		switch n := (*ptr).(type) {
		case *leafNode:
			leaf := (*ptr).(*leafNode)
			return leaf
		case *branchNode:
			bn := (*ptr).(*branchNode)
			offsetMax := bn.twigOffsetMax()
			ptr = n.twig(offsetMax - 1)
		}
	}
}

// GetLessOrEqual returns the key-value pair with the largest key that is less than or equal to
// the given key. It returns the key, value, and a boolean indicating whether an exact match was found.
// If no such key exists, it returns nil, nil, false.
func (tr *Trie) GetLessOrEqual(key []byte) (k []byte, v any, exactMatch bool) {
	must(key)

	if tr.root == nil {
		return nil, nil, false
	}

	leaf := tr.findMatch(key, false)
	if leaf != nil && bytes.Equal(key, leaf.key) {
		return key, leaf.value, true
	}

	index, match := nibbleIndex(key, leaf.key)
	if match {
		panic(errInternal)
	}

	prev, cur, needCheckCur := tr.findPrev(index, key)
	if needCheckCur {
		b1 := nibbleBit(index, key)
		b2 := nibbleBit(index, leaf.key)
		if b1 > b2 {
			leaf = tr.lastLeaf(cur)
			return leaf.key, leaf.value, false
		}
	}

	if prev == nil {
		return nil, nil, false
	}
	leaf = tr.lastLeaf(prev)
	return leaf.key, leaf.value, false
}

func must(key []byte) {
	if len(key) == 0 {
		panic(errKeyEmpty)
	}
	if len(key) > maxKeyBytes {
		panic(errKeyTooLong)
	}
}
