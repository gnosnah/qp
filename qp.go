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
// It must not modify the trie or transaction that calls it.
type OnInsertValFn[V any] = func(newVal V) (finalVal V)

// OnUpdateValFn is a function type that takes two parameters of type V (newVal and oldVal)
// and returns a finalVal of type V. It is used to handle value updates by comparing
// the new and old values and determining the final value to be used.
// It must not modify the trie or transaction that calls it, and it
// must return a new value instead of changing oldVal in place.
type OnUpdateValFn[V any] = func(newVal, oldVal V) (finalVal V)

const (
	nibbleIndexMax = math.MaxUint16
	maxKeyBytes    = nibbleIndexMax >> 1
)

var (
	errKeyEmpty   = fmt.Errorf("empty key")
	errKeyTooLong = fmt.Errorf("max key length is %d bytes", maxKeyBytes)
	errInternal   = fmt.Errorf("internal error")
	// errBadNode is the panic value when a walk meets a node that is neither a
	// leaf nor a branch. The trie is corrupted, most likely by writes from
	// several goroutines or by breaking the transaction rules.
	errBadNode = fmt.Errorf("internal error: nil node, the trie is corrupted")
)

// default newVal is finalVal
func defaultOnInsert[V any](newVal V) V { return newVal }

func defaultOnUpdate[V any](newVal, oldVal V) V { return newVal }

type Option[V any] func(*Trie[V])

func WithOnInsert[V any](f OnInsertValFn[V]) Option[V] {
	return func(tr *Trie[V]) {
		tr.onInsert = f
	}
}

func WithOnUpdate[V any](f OnUpdateValFn[V]) Option[V] {
	return func(tr *Trie[V]) {
		tr.onUpdate = f
	}
}

// Trie is a qp-trie that maps byte-string keys to values of type V.
// Create one with New. The zero value is not usable.
// A Trie must not be copied by value after initialization; pass *Trie instead.
// Use Txn to create a new version with copy-on-write updates.
type Trie[V any] struct {
	root     trieNode[V]
	size     int
	onInsert OnInsertValFn[V]
	onUpdate OnUpdateValFn[V]
}

// New creates and initializes a new Trie with the given options.
// If no onInsert or onUpdate handlers are provided, default handlers will be used.
// default onInsert: func(newVal V) V { return newVal }
// default onUpdate: func(newVal, oldVal V) V { return newVal }
//
// The value type cannot be inferred without options, so it is given
// explicitly: New[int](). With options it is inferred from the handlers.
func New[V any](opts ...Option[V]) *Trie[V] {
	var tr Trie[V]
	for _, opt := range opts {
		opt(&tr)
	}
	if tr.onInsert == nil {
		tr.onInsert = defaultOnInsert[V]
	}
	if tr.onUpdate == nil {
		tr.onUpdate = defaultOnUpdate[V]
	}
	return &tr
}

// Size returns the total number of key-value pairs stored in the trie.
func (tr *Trie[V]) Size() int {
	return tr.size
}

func (tr *Trie[V]) findMatch(key []byte, exactMatch bool) *leafNode[V] {
	if tr.root == nil {
		return nil
	}
	var bn *branchNode[V]
	ptr := &tr.root
	for {
		switch n := (*ptr).(type) {
		case *leafNode[V]:
			return n
		case *branchNode[V]:
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
		default:
			panic(errBadNode)
		}
	}
}

func (tr *Trie[V]) findInsert(key []byte, index nibbleIndexT) (ptr *trieNode[V], grow bool) {
	ptr = &tr.root
	for {
		switch n := (*ptr).(type) {
		case *leafNode[V]:
			return ptr, false
		case *branchNode[V]:
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
		default:
			panic(errBadNode)
		}
	}
}

func (tr *Trie[V]) findDelete(key []byte) (parentBranch *trieNode[V], leaf *leafNode[V], b bitmapT) {
	if tr.root == nil {
		return nil, nil, 0
	}

	ptr := &tr.root
	for {
		switch n := (*ptr).(type) {
		case *leafNode[V]:
			return parentBranch, n, b
		case *branchNode[V]:
			b = n.twigBit(key)
			if !n.hasTwig(b) {
				return nil, nil, 0
			}
			i := n.twigOffset(b)
			parentBranch = ptr
			ptr = n.twig(i)
		default:
			panic(errBadNode)
		}
	}
}

// Get retrieves the value associated with the given key from the trie.
// It returns the value and a boolean indicating whether the key was found.
// If the key is not present in the trie, it returns the zero value of V and false.
func (tr *Trie[V]) Get(key []byte) (val V, found bool) {
	must(key)
	var zero V
	leaf := tr.findMatch(key, true)
	if leaf != nil && bytes.Equal(key, leaf.key) {
		return leaf.value, true
	}
	return zero, false
}

// Upsert inserts or updates a key-value pair in the trie.
// If the key already exists, it updates the value and returns the old value with isUpdate=true.
// If the key does not exist, it inserts the new key-value pair and returns the
// zero value of V with isUpdate=false.
func (tr *Trie[V]) Upsert(key []byte, value V) (oldVal V, isUpdate bool) {
	must(key)
	var zero V

	if tr.root == nil {
		tr.root = newLeafNode(key, tr.onInsert(value))
		tr.size++
		return zero, false
	}

	leaf := tr.findMatch(key, false)
	index, match := nibbleIndex(key, leaf.key)
	if match {
		preValue := leaf.value
		leaf.value = tr.onUpdate(value, preValue)
		return preValue, true
	}

	newLeaf := newLeafNode(key, tr.onInsert(value))
	ptr, grow := tr.findInsert(key, index)
	if grow {
		bn := (*ptr).(*branchNode[V])
		bn.growTwigs(index, key, newLeaf)
	} else {
		bn := newBranchNode(*ptr, index, leaf.key, key, newLeaf)
		*ptr = bn
	}

	tr.size++
	return zero, false
}

// Delete removes the entry for the given key from the trie.
// It returns the value that was associated with the key and a boolean indicating
// whether the key was present in the trie.
// If the key is not found, it returns the zero value of V and false.
func (tr *Trie[V]) Delete(key []byte) (oldVal V, found bool) {
	must(key)
	var zero V

	parentBn, leaf, b := tr.findDelete(key)
	if leaf == nil || !bytes.Equal(key, leaf.key) {
		return zero, false
	}
	tr.size--
	if parentBn == nil {
		// only when root is leafNode
		tr.root = nil
		return leaf.value, true
	}

	bn := (*parentBn).(*branchNode[V])
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

func (tr *Trie[V]) findPrev(index nibbleIndexT, key []byte) (prev *trieNode[V], cur *trieNode[V], needCheckCur bool) {
	cur = &tr.root
	for {
		switch n := (*cur).(type) {
		case *leafNode[V]:
			needCheckCur = true
			return
		case *branchNode[V]:
			bn := (*cur).(*branchNode[V])
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
		default:
			panic(errBadNode)
		}
	}
}

func (tr *Trie[V]) lastLeaf(node *trieNode[V]) *leafNode[V] {
	ptr := node
	for {
		switch n := (*ptr).(type) {
		case *leafNode[V]:
			leaf := (*ptr).(*leafNode[V])
			return leaf
		case *branchNode[V]:
			bn := (*ptr).(*branchNode[V])
			offsetMax := bn.twigOffsetMax()
			ptr = n.twig(offsetMax - 1)
		default:
			panic(errBadNode)
		}
	}
}

// GetLessOrEqual returns the key-value pair with the largest key that is less than or equal to
// the given key. found reports whether such a key exists, and exactMatch reports whether it
// equals the given key.
// If no such key exists, it returns a nil key, the zero value of V, false, and false.
// The returned key belongs to the trie and must not be modified by the caller.
// A caller-owned query buffer may be reused after the call returns.
func (tr *Trie[V]) GetLessOrEqual(key []byte) (k []byte, v V, exactMatch bool, found bool) {
	must(key)
	var zero V

	if tr.root == nil {
		return nil, zero, false, false
	}

	leaf := tr.findMatch(key, false)
	if leaf != nil && bytes.Equal(key, leaf.key) {
		return leaf.key, leaf.value, true, true
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
			return leaf.key, leaf.value, false, true
		}
	}

	if prev == nil {
		return nil, zero, false, false
	}
	leaf = tr.lastLeaf(prev)
	return leaf.key, leaf.value, false, true
}

func must(key []byte) {
	if len(key) == 0 {
		panic(errKeyEmpty)
	}
	if len(key) > maxKeyBytes {
		panic(errKeyTooLong)
	}
}
