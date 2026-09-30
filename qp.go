package qp

import (
	"bytes"
)

// OnInsertValFn is a function type that processes a new value before insertion.
// It takes the new value as input and returns the final value to be used.
type OnInsertValFn = func(newVal any) (finalVal any)

// OnUpdateValFn is a function type that takes two parameters of type any (newVal and oldVal)
// and returns a finalVal of type any. It is used to handle value updates by comparing
// the new and old values and determining the final value to be used.
type OnUpdateValFn = func(newVal, oldVal any) (finalVal any)

type bitmapT = uint32      // bitmap type, 17 bits, first bit NO_BYTE
type nibbleIndexT = uint16 // nibble index type

var (
	// default newVal is finalVal
	defaultOnInsert = func(newVal any) any { return newVal }
	defaultOnUpdate = func(newVal, oldVal any) any { return newVal }
)

// Option configures a Trie at construction time.
type Option func(*Trie)

// WithOnInsert sets a hook applied to values of newly inserted keys.
func WithOnInsert(f OnInsertValFn) Option {
	return func(tr *Trie) {
		tr.onInsert = f
	}
}

// WithOnUpdate sets a hook applied when an existing key is updated.
func WithOnUpdate(f OnUpdateValFn) Option {
	return func(tr *Trie) {
		tr.onUpdate = f
	}
}

// Trie is an ordered in-memory map from byte-slice keys to values.
// It is not safe for concurrent use.
type Trie struct {
	root     trieNode
	size     int
	onInsert OnInsertValFn
	onUpdate OnUpdateValFn
	txn      txnRole
	family   *txnFamily
}

// txnRole records whether this trie is idle, the origin of an open
// transaction, or that transaction's working copy.
type txnRole uint8

const (
	txnIdle txnRole = iota
	txnOrigin
	txnWork
)

// txnFamily is shared by a trie and every snapshot committed from it.
// At most one transaction may be open in a family.
type txnFamily struct {
	open bool
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
	tr.family = &txnFamily{}
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
	ptr := &tr.root
	for {
		switch n := (*ptr).(type) {
		case *leafNode:
			return n
		case *branchNode:
			i := 0
			b := n.twigBit(key)
			if n.hasTwig(b) {
				i = n.twigOffset(b)
			} else if exactMatch {
				return nil
			}
			ptr = n.twig(i)
		}
	}
}

// findInsert walks down to the node the new leaf has to be attached to, or to
// the leaf already holding key when exactMatch is true. Every node shared with
// another trie is replaced by a private copy on the way down, so that the
// caller may modify the returned node in place.
func (tr *Trie) findInsert(key []byte, index nibbleIndexT, exactMatch bool) (ptr *trieNode, grow bool) {
	ptr = &tr.root
	for {
		switch n := uncow(ptr).(type) {
		case *leafNode:
			return ptr, false
		case *branchNode:
			if !exactMatch {
				if index == n.index {
					return ptr, true
				}
				if index < n.index {
					return ptr, false
				}
			}

			b := n.twigBit(key)
			if !n.hasTwig(b) {
				panic(errInternal)
			}
			ptr = n.twig(n.twigOffset(b))
		}
	}
}

// findDelete walks down to the leaf key would be stored in, copying every node
// shared with another trie on the way down. It returns the branch the leaf
// hangs off and the bit the leaf occupies in that branch.
func (tr *Trie) findDelete(key []byte) (parentBranch *trieNode, leaf *leafNode, b bitmapT) {
	if tr.root == nil {
		return nil, nil, 0
	}

	ptr := &tr.root
	for {
		switch n := uncow(ptr).(type) {
		case *leafNode:
			return parentBranch, n, b
		case *branchNode:
			b = n.twigBit(key)
			if !n.hasTwig(b) {
				panic(errInternal)
			}
			i := n.twigOffset(b)
			parentBranch = ptr
			ptr = n.twig(i)
		}
	}
}

// Get retrieves the value associated with the given key.
// If the key is not present, it returns nil, false, nil.
func (tr *Trie) Get(key []byte) (val any, found bool, err error) {
	if err = checkKey(key); err != nil {
		return nil, false, err
	}
	leaf := tr.findMatch(key, true)
	if leaf != nil && bytes.Equal(key, leaf.key) {
		return leaf.value, true, nil
	}
	return nil, false, nil
}

// Upsert inserts or updates a key-value pair.
// If the key already exists, it updates the value and returns the old value with isUpdate=true.
// If the key does not exist, it inserts the new pair and returns nil, false, nil.
//
// The key is copied; the caller may reuse or modify the slice afterwards.
func (tr *Trie) Upsert(key []byte, value any) (oldVal any, isUpdate bool, err error) {
	if err = checkKey(key); err != nil {
		return nil, false, err
	}
	if err = tr.checkWrite(); err != nil {
		return nil, false, err
	}

	if tr.root == nil {
		tr.root = &leafNode{key: bytes.Clone(key), value: tr.onInsert(value)}
		tr.size++
		return nil, false, nil
	}

	// locate a leaf to compute the divergence nibble, then copy the write path
	leaf := tr.findMatch(key, false)
	index, exactMatch := nibbleIndex(key, leaf.key)
	if exactMatch {
		oldVal = leaf.value
	}

	ptr, grow := tr.findInsert(key, index, exactMatch)
	if exactMatch {
		lf := (*ptr).(*leafNode)
		lf.value = tr.onUpdate(value, oldVal)
		return oldVal, true, nil
	}

	newLeaf := &leafNode{key: bytes.Clone(key), value: tr.onInsert(value)}
	if grow {
		bn := (*ptr).(*branchNode)
		bn.growTwigs(index, key, newLeaf)
	} else {
		bn := newBranchNode(*ptr, index, leaf.key, key, newLeaf)
		*ptr = bn
	}

	tr.size++
	return nil, false, nil
}

// Delete removes the entry for the given key.
// If the key is not found, it returns nil, false, nil.
func (tr *Trie) Delete(key []byte) (oldVal any, found bool, err error) {
	if err = checkKey(key); err != nil {
		return nil, false, err
	}
	if err = tr.checkWrite(); err != nil {
		return nil, false, err
	}

	hit := tr.findMatch(key, true)
	if hit == nil || !bytes.Equal(key, hit.key) {
		return nil, false, nil
	}

	parentBn, leaf, b := tr.findDelete(key)
	if leaf == nil || !bytes.Equal(key, leaf.key) {
		panic(errInternal)
	}
	tr.size--
	if parentBn == nil {
		tr.root = nil
		return leaf.value, true, nil
	}

	bn := (*parentBn).(*branchNode)
	if bn.twigOffsetMax() == 2 {
		other := 0
		if bn.twigOffset(b) == 0 {
			other = 1
		}
		otherTwig := bn.twig(other)
		*parentBn = *otherTwig
		return leaf.value, true, nil
	}

	bn.removeTwig(b)
	return leaf.value, true, nil
}

func (tr *Trie) findPrev(index nibbleIndexT, key []byte) (prev *trieNode, cur *trieNode, needCheckCur bool) {
	cur = &tr.root
	for {
		switch n := (*cur).(type) {
		case *leafNode:
			needCheckCur = true
			return
		case *branchNode:
			if index < n.index {
				needCheckCur = true
				return
			}
			b := n.twigBit(key)
			i := n.twigOffset(b)
			if i > 0 {
				prev = n.twig(i - 1)
			}
			if index == n.index {
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
			return n
		case *branchNode:
			ptr = n.twig(n.twigOffsetMax() - 1)
		}
	}
}

// GetLessOrEqual returns the key-value pair with the largest key that is
// less than or equal to the given key. If no such key exists, it returns
// nil, nil, false, nil. The returned key must not be modified.
func (tr *Trie) GetLessOrEqual(key []byte) (k []byte, v any, exactMatch bool, err error) {
	if err = checkKey(key); err != nil {
		return nil, nil, false, err
	}

	if tr.root == nil {
		return nil, nil, false, nil
	}

	leaf := tr.findMatch(key, false)
	if leaf != nil && bytes.Equal(key, leaf.key) {
		return key, leaf.value, true, nil
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
			return leaf.key, leaf.value, false, nil
		}
	}

	if prev == nil {
		return nil, nil, false, nil
	}
	leaf = tr.lastLeaf(prev)
	return leaf.key, leaf.value, false, nil
}

func (tr *Trie) checkWrite() error {
	if tr.txn == txnWork {
		return nil
	}
	if tr.txn == txnOrigin {
		return ErrTrieFrozen
	}
	if tr.family != nil && tr.family.open {
		return ErrTrieFrozen
	}
	return nil
}
