package qp

import (
	"bytes"
	"testing"
)

func Test_KeyLimits(t *testing.T) {
	keys := []struct {
		name string
		key  []byte
		want error
	}{
		{"nil", nil, errKeyEmpty},
		{"empty", []byte{}, errKeyEmpty},
		{"too long", bytes.Repeat([]byte{'a'}, maxKeyBytes+1), errKeyTooLong},
		{"length wraps uint16", bytes.Repeat([]byte{'a'}, 1<<16), errKeyTooLong},
	}
	for _, key := range keys {
		t.Run(key.name, func(t *testing.T) {
			for _, kind := range []string{"trie", "transaction"} {
				t.Run(kind, func(t *testing.T) {
					tr := New[int]()
					tr.Upsert([]byte("a"), 1)
					var w trieWriter = tr
					if kind == "transaction" {
						tx := tr.Txn()
						defer tx.Abort()
						w = tx
					}
					operations := []struct {
						name string
						fn   func()
					}{
						{"Get", func() { w.Get(key.key) }},
						{"Upsert", func() { w.Upsert(key.key, 2) }},
						{"Delete", func() { w.Delete(key.key) }},
					}
					if kind == "trie" {
						operations = append(operations, struct {
							name string
							fn   func()
						}{"GetLessOrEqual", func() { tr.GetLessOrEqual(key.key) }})
					}
					for _, op := range operations {
						t.Run(op.name, func(t *testing.T) {
							defer func() {
								if got := recover(); got != key.want {
									t.Errorf("panic = %v, want %v", got, key.want)
								}
							}()
							op.fn()
						})
					}
					if got, found := w.Get([]byte("a")); !found || got != 1 {
						t.Fatalf("valid entry changed: Get(a) = %d, %v", got, found)
					}
					checkModel(t, "base", tr, trieModel{"a": 1}, nil)
				})
			}
		})
	}
}
