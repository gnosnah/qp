package qp

import "bytes"

type Txn[V any] struct {
	oldTr *Trie[V]
	newTr *Trie[V]
}

// Txn creates a new transaction for the Trie. It returns a transaction object
// that provides copy-on-write functionality for modifying the trie. The original
// trie remains unchanged until the transaction is committed.
//
// A trie must not be modified directly while a transaction is open. Only one
// transaction may be open at a time across this trie and every version that
// shares nodes with it (OldTrie, NewTrie, and tries returned by an earlier
// Commit or Abort).
//
// Other goroutines may keep reading the original trie while the transaction
// is open. Values are not copied: a stored value is shared by every version
// that holds its key and must be treated as immutable.
func (tr *Trie[V]) Txn() *Txn[V] {
	var tx Txn[V]
	tx.newTr = &Trie[V]{
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

func (tx *Txn[V]) OldTrie() *Trie[V] {
	return tx.oldTr
}

func (tx *Txn[V]) NewTrie() *Trie[V] {
	return tx.newTr
}

// Commit finalizes the transaction by setting the old trie to the new trie
// and clearing the new trie reference. Returns the committed trie.
func (tx *Txn[V]) Commit() *Trie[V] {
	tx.oldTr = tx.newTr
	tx.newTr = nil
	return tx.oldTr
}

// Abort cancels the transaction and returns the original trie state.
// Any changes made during the transaction will be discarded.
func (tx *Txn[V]) Abort() *Trie[V] {
	tx.newTr = nil
	return tx.oldTr
}

// Get retrieves a value associated with the given key from the transaction.
// It returns the value and a boolean indicating whether the key was found.
// If the key is not present, it returns the zero value of V and false.
func (tx *Txn[V]) Get(key []byte) (val V, found bool) {
	return tx.newTr.Get(key)
}

// Upsert inserts a new key-value pair or updates an existing one in the transaction.
// It returns the old value if the key existed (update case) and a boolean indicating
// whether it was an update operation. For new insertions, it returns the zero
// value of V and false.
// The key must not be empty.
func (tx *Txn[V]) Upsert(key []byte, value V) (oldVal V, isUpdate bool) {
	must(key)
	var zero V

	if tx.newTr.root == nil {
		tx.newTr.root = newLeafNode(key, tx.newTr.onInsert(value))
		tx.newTr.size++
		return zero, false
	}

	leaf := tx.newTr.findMatch(key, false)
	index, exactMatch := nibbleIndex(key, leaf.key)
	if exactMatch {
		oldVal = leaf.value
	}
	ptr, growBranch := tx.findInsert(key, index, exactMatch)
	if exactMatch {
		lf := (*ptr).(*leafNode[V])
		lf.value = tx.newTr.onUpdate(value, oldVal)
		return oldVal, true
	}

	newLeaf := newLeafNode(key, tx.newTr.onInsert(value))
	if growBranch {
		bn := (*ptr).(*branchNode[V])
		bn.growTwigs(index, key, newLeaf)
	} else {
		bn := newBranchNode(*ptr, index, leaf.key, key, newLeaf)
		*ptr = bn
	}

	tx.newTr.size++
	return zero, false
}

// findInsert and findDelete copy every cow-marked node on the way down and
// continue in the copy. The mark on the original node is left set: that node
// still belongs to the other versions, so it must never look writable again.
func (tx *Txn[V]) findInsert(key []byte, index nibbleIndexT, exactMatch bool) (ptr *trieNode[V], growBranch bool) {
	ptr = &tx.newTr.root
	for {
		switch n := (*ptr).(type) {
		case *leafNode[V]:
			if n.cowMarked() {
				newLf := n.dup()
				*ptr = newLf
			}
			return ptr, false
		case *branchNode[V]:
			if n.cowMarked() {
				n.markTwigs()
				newBn := n.dup()
				*ptr = newBn
				n = newBn.(*branchNode[V])
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
		default:
			panic(errBadNode)
		}
	}
}

func (tx *Txn[V]) findDelete(key []byte) (parentBn *trieNode[V], leaf *leafNode[V], b bitmapT) {
	ptr := &tx.newTr.root
	for {
		switch n := (*ptr).(type) {
		case *leafNode[V]:
			if n.cowMarked() {
				newLf := n.dup()
				*ptr = newLf
			}
			return parentBn, n, b
		case *branchNode[V]:
			if n.cowMarked() {
				n.markTwigs()
				newBn := n.dup()
				*ptr = newBn
				n = newBn.(*branchNode[V])
			}
			b = n.twigBit(key)
			if !n.hasTwig(b) {
				return nil, nil, 0
			}
			i := n.twigOffset(b)
			parentBn = ptr
			ptr = n.twig(i)
		default:
			panic(errBadNode)
		}
	}
}

// Delete removes the entry for the given key from the transaction.
// It returns the old value and true if the key was present, or the zero value
// of V and false if not found.
// The key must not be empty.
func (tx *Txn[V]) Delete(key []byte) (oldVal V, found bool) {
	must(key)
	var zero V

	if tx.newTr.root == nil {
		return zero, false
	}

	parentBn, leaf, b := tx.findDelete(key)
	if leaf == nil || !bytes.Equal(key, leaf.key) {
		return zero, false
	}
	tx.newTr.size--
	if parentBn == nil {
		tx.newTr.root = nil
		return leaf.value, true
	}

	bn := (*parentBn).(*branchNode[V])
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
