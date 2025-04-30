package qp

import "bytes"

type Txn struct {
	oldTr *Trie
	newTr *Trie
}

// Txn creates a new transaction for the Trie. It returns a transaction object
// that provides copy-on-write functionality for modifying the trie. The original
// trie remains unchanged until the transaction is committed.
func (tr *Trie) Txn() *Txn {
	var tx Txn
	tx.newTr = &Trie{
		root:     tr.root,
		size:     tr.size,
		onInsert: tr.onInsert,
		onUpdate: tr.onUpdate,
	}
	if tr.root != nil {
		tx.newTr.root.markCow()
	}

	tx.oldTr = tr
	return &tx
}

// Commit finalizes the transaction by setting the old trie to the new trie
// and clearing the new trie reference. Returns the committed trie.
func (tx *Txn) Commit() *Trie {
	tx.oldTr = tx.newTr
	tx.newTr = nil
	return tx.oldTr
}

// Abort cancels the transaction and returns the original trie state.
// Any changes made during the transaction will be discarded.
func (tx *Txn) Abort() *Trie {
	tx.newTr = nil
	return tx.oldTr
}

// Get retrieves a value associated with the given key from the transaction.
// It returns the value and a boolean indicating whether the key was found.
func (tx *Txn) Get(key []byte) (val any, found bool) {
	return tx.newTr.Get(key)
}

// Upsert inserts a new key-value pair or updates an existing one in the transaction.
// It returns the old value if the key existed (update case) and a boolean indicating
// whether it was an update operation. For new insertions, it returns nil and false.
// The key must not be nil.
func (tx *Txn) Upsert(key []byte, value any) (oldVal any, isUpdate bool) {
	must(key)

	newLeaf := &leafNode{key: key, value: tx.newTr.onInsert(value), cow: false}

	if tx.newTr.root == nil {
		tx.newTr.root = newLeaf
		tx.newTr.size++
		return nil, false
	}

	leaf := tx.newTr.findMatch(key, false)
	index, exactMatch := nibbleIndex(key, leaf.key)
	if exactMatch {
		oldVal = leaf.value
	}
	ptr, growBranch := tx.findInsert(key, index, exactMatch)
	if exactMatch {
		lf := (*ptr).(*leafNode)
		lf.value = tx.newTr.onUpdate(value, oldVal)
		return oldVal, true
	}

	if growBranch {
		bn := (*ptr).(*branchNode)
		bn.growTwigs(index, key, newLeaf)
	} else {
		bn := newBranchNode(*ptr, index, leaf.key, key, newLeaf)
		*ptr = bn
	}

	tx.newTr.size++
	return nil, false
}

func (tx *Txn) findInsert(key []byte, index nibbleIndexT, exactMatch bool) (ptr *trieNode, growBranch bool) {
	ptr = &tx.newTr.root
	for {
		switch n := (*ptr).(type) {
		case *leafNode:
			if n.cowMarked() {
				n.clearCow()
				newLf := n.dup()
				*ptr = newLf
			}
			return ptr, false
		case *branchNode:
			if n.cowMarked() {
				n.markTwigs()
				n.clearCow()
				newBn := n.dup()
				*ptr = newBn
				n = newBn.(*branchNode)
			}
			if !exactMatch {
				if index == n.index {
					return ptr, true
				}
				if index < n.index {
					return ptr, false
				}
			}

			i := 0
			b := n.twigBit(key)
			if n.hasTwig(b) {
				i = n.twigOffset(b)
			}
			ptr = n.twig(i)
		}
	}
}

func (tx *Txn) findDelete(key []byte) (parentBn *trieNode, leaf *leafNode, b bitmapT) {
	ptr := &tx.newTr.root
	for {
		switch n := (*ptr).(type) {
		case *leafNode:
			if n.cowMarked() {
				n.clearCow()
				newLf := n.dup()
				*ptr = newLf
			}
			return parentBn, n, b
		case *branchNode:
			if n.cowMarked() {
				n.markTwigs()
				n.clearCow()
				newBn := n.dup()
				*ptr = newBn
				n = newBn.(*branchNode)
			}
			b = n.twigBit(key)
			if !n.hasTwig(b) {
				return nil, nil, 0
			}
			i := n.twigOffset(b)
			parentBn = ptr
			ptr = n.twig(i)
		}
	}
}

// Delete removes the entry for the given key from the transaction.
// It returns the old value and true if the key was present, or nil and false if not found.
// The key must not be nil.
func (tx *Txn) Delete(key []byte) (oldVal any, found bool) {
	must(key)

	if tx.newTr.root == nil {
		return nil, false
	}

	parentBn, leaf, b := tx.findDelete(key)
	if leaf == nil || !bytes.Equal(key, leaf.key) {
		return nil, false
	}
	tx.newTr.size--
	if parentBn == nil {
		tx.newTr.root = nil
		return leaf.value, true
	}

	bn := (*parentBn).(*branchNode)
	if bn.twigOffsetMax() == 2 {
		other := 0
		if bn.twigOffset(b) == 0 {
			other = 1
		}
		otherTwig := bn.twig(other)
		*parentBn = *otherTwig
		return leaf.value, true
	}

	bn.removeTwig(b)
	return leaf.value, true
}
