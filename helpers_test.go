package qp

import "testing"

func upsert(t testing.TB, tr *Trie, k []byte, v any) (any, bool) {
	t.Helper()
	old, upd, err := tr.Upsert(k, v)
	if err != nil {
		t.Fatal(err)
	}
	return old, upd
}

func get(t testing.TB, tr *Trie, k []byte) (any, bool) {
	t.Helper()
	v, ok, err := tr.Get(k)
	if err != nil {
		t.Fatal(err)
	}
	return v, ok
}

func del(t testing.TB, tr *Trie, k []byte) (any, bool) {
	t.Helper()
	v, ok, err := tr.Delete(k)
	if err != nil {
		t.Fatal(err)
	}
	return v, ok
}

func begin(t *testing.T, tr *Trie) *Txn {
	t.Helper()
	tx, err := tr.Txn()
	if err != nil {
		t.Fatal(err)
	}
	return tx
}

func txUpsert(t *testing.T, tx *Txn, k []byte, v any) (any, bool) {
	t.Helper()
	old, upd, err := tx.Upsert(k, v)
	if err != nil {
		t.Fatal(err)
	}
	return old, upd
}

func txGet(t *testing.T, tx *Txn, k []byte) (any, bool) {
	t.Helper()
	v, ok, err := tx.Get(k)
	if err != nil {
		t.Fatal(err)
	}
	return v, ok
}

func txDel(t *testing.T, tx *Txn, k []byte) (any, bool) {
	t.Helper()
	v, ok, err := tx.Delete(k)
	if err != nil {
		t.Fatal(err)
	}
	return v, ok
}

func gle(t *testing.T, tr *Trie, k []byte) ([]byte, any, bool) {
	t.Helper()
	gk, v, exact, err := tr.GetLessOrEqual(k)
	if err != nil {
		t.Fatal(err)
	}
	return gk, v, exact
}
