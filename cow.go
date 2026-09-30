package qp

// Txn is a copy-on-write transaction against a Trie.
//
// Only one transaction may be open at a time in a snapshot family
// (a trie and the tries committed from it). Nested transactions and
// concurrent transactions are not supported. Txn is not safe for concurrent use.
type Txn struct {
	oldTr *Trie
	newTr *Trie
}

// Txn starts a copy-on-write transaction.
//
// At most one transaction may be open in the snapshot family. Calling Txn
// again on this trie, on a sibling snapshot, on the working copy, or via
// NewTrie returns ErrTxnOpen until Commit or Abort.
//
// While the transaction is open, every trie in the family is read-only
// except the working copy. After Commit or Abort the family may be written
// again and may start a new transaction. Distinct New() tries have separate
// families and may each have their own transaction.
func (tr *Trie) Txn() (*Txn, error) {
	if tr.family == nil {
		tr.family = &txnFamily{}
	}
	if tr.family.open {
		return nil, ErrTxnOpen
	}

	newTr := &Trie{
		root:     tr.root,
		size:     tr.size,
		onInsert: tr.onInsert,
		onUpdate: tr.onUpdate,
		txn:      txnWork,
		family:   tr.family,
	}
	if tr.root != nil {
		newTr.root.markCow()
	}

	tr.family.open = true
	tr.txn = txnOrigin
	return &Txn{oldTr: tr, newTr: newTr}, nil
}

// OldTrie returns the origin trie. While the transaction is open it is read-only.
func (tx *Txn) OldTrie() *Trie {
	return tx.oldTr
}

// NewTrie returns the working copy holding modifications made so far.
// It returns ErrTxnFinished after Commit or Abort.
// The returned trie cannot start a nested transaction.
func (tx *Txn) NewTrie() (*Trie, error) {
	return tx.trie()
}

func (tx *Txn) trie() (*Trie, error) {
	if tx.newTr == nil {
		return nil, ErrTxnFinished
	}
	return tx.newTr, nil
}

func (tx *Txn) end() {
	if tx.newTr == nil {
		return
	}
	tx.oldTr.txn = txnIdle
	tx.newTr.txn = txnIdle
	if tx.oldTr.family != nil {
		tx.oldTr.family.open = false
	}
}

// Commit applies the transaction and returns the committed trie.
// Calling Commit again returns the same trie.
func (tx *Txn) Commit() *Trie {
	if tx.newTr != nil {
		tx.end()
		tx.oldTr = tx.newTr
		tx.newTr = nil
	}
	return tx.oldTr
}

// Abort discards the transaction and returns the origin trie.
func (tx *Txn) Abort() *Trie {
	tx.end()
	tx.newTr = nil
	return tx.oldTr
}

// Get retrieves a value from the transaction's working copy.
func (tx *Txn) Get(key []byte) (val any, found bool, err error) {
	tr, err := tx.trie()
	if err != nil {
		return nil, false, err
	}
	return tr.Get(key)
}

// Upsert inserts or updates a key-value pair in the transaction's working copy.
func (tx *Txn) Upsert(key []byte, value any) (oldVal any, isUpdate bool, err error) {
	tr, err := tx.trie()
	if err != nil {
		return nil, false, err
	}
	return tr.Upsert(key, value)
}

// Delete removes a key from the transaction's working copy.
func (tx *Txn) Delete(key []byte) (oldVal any, found bool, err error) {
	tr, err := tx.trie()
	if err != nil {
		return nil, false, err
	}
	return tr.Delete(key)
}
