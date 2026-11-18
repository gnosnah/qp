package qp

import "testing"

func Test_TxnClosed(t *testing.T) {
	finishers := []struct {
		name string
		fn   func(*Txn[int]) *Trie[int]
	}{
		{"commit", (*Txn[int]).Commit},
		{"abort", (*Txn[int]).Abort},
	}
	operations := []struct {
		name string
		fn   func(*Txn[int])
	}{
		{"Commit", func(tx *Txn[int]) { tx.Commit() }},
		{"Abort", func(tx *Txn[int]) { tx.Abort() }},
		{"OldTrie", func(tx *Txn[int]) { tx.OldTrie() }},
		{"NewTrie", func(tx *Txn[int]) { tx.NewTrie() }},
		{"Get", func(tx *Txn[int]) { tx.Get([]byte("a")) }},
		{"Upsert", func(tx *Txn[int]) { tx.Upsert([]byte("a"), 3) }},
		{"Delete", func(tx *Txn[int]) { tx.Delete([]byte("a")) }},
	}
	for _, finish := range finishers {
		for _, op := range operations {
			t.Run(finish.name+"/"+op.name, func(t *testing.T) {
				tr := New[int]()
				tr.Upsert([]byte("a"), 1)
				tx := tr.Txn()
				if tx.OldTrie() != tr {
					t.Fatal("OldTrie did not return the original trie")
				}
				tx.Upsert([]byte("a"), 2)
				next := tx.NewTrie()
				got := finish.fn(tx)
				want := next
				if finish.name == "abort" {
					want = tr
				}
				if got != want {
					t.Fatal("transaction returned the wrong trie")
				}

				func() {
					defer func() {
						p := recover()
						err, ok := p.(error)
						if !ok || err.Error() != "transaction is not open" {
							t.Errorf("panic = %v; want transaction is not open", p)
						}
					}()
					op.fn(tx)
				}()

				// Rejected operations must leave both saved versions usable.
				checkModel(t, "original", tr, trieModel{"a": 1}, nil)
				checkModel(t, "new version", next, trieModel{"a": 2}, nil)
			})
		}
	}
}

func Test_TxnEmptyFinish(t *testing.T) {
	tr := New[int]()
	tx := tr.Txn()
	next := tx.Commit()
	checkModel(t, "empty commit", next, trieModel{}, nil)
	tx = next.Txn()
	tx.Upsert([]byte("a"), 1)
	if got := tx.Abort(); got != next {
		t.Fatal("Abort did not return the empty base trie")
	}
	checkModel(t, "empty abort", next, trieModel{}, nil)
	checkModel(t, "original", tr, trieModel{}, nil)
}
