package qp

import (
	"bytes"
	"reflect"
	"testing"
)

func Test_CowMaxKeyAndWideBranches(t *testing.T) {
	tr, m := New[int](), trieModel{}
	prefix := bytes.Repeat([]byte{0xff}, maxKeyBytes-1)
	keys := [][]byte{prefix}
	for n := range 256 {
		keys = append(keys, append(bytes.Clone(prefix), byte(n)))
	}
	for i, k := range keys {
		tr.Upsert(k, i)
		m[string(k)] = i
	}
	if width, _ := trieShape(tr.root); width != 17 {
		t.Fatalf("widest branch has %d twigs, want 17", width)
	}
	checkModel(t, "max keys initial", tr, m, [][]byte{{0}, {0xff}})

	tx := tr.Txn()
	nm := m.clone()
	for i, k := range keys {
		if i%3 == 0 {
			tx.Delete(k)
			delete(nm, string(k))
		} else {
			tx.Upsert(k, -i)
			nm[string(k)] = -i
		}
	}
	next := tx.Commit()
	checkModel(t, "max keys old", tr, m, nil)
	checkModel(t, "max keys new", next, nm, nil)

	// Deleting every key from an older version must preserve both snapshots,
	// including when branches collapse and the root finally becomes nil.
	tx = tr.Txn()
	for _, k := range keys {
		tx.Delete(k)
	}
	checkModel(t, "max keys deleted", tx.NewTrie(), trieModel{}, nil)
	tx.Abort()
	checkModel(t, "max keys after abort", tr, m, nil)
	checkModel(t, "max keys preserved sibling", next, nm, nil)
}

func Test_CowCollapseThenUpdate(t *testing.T) {
	tests := []struct {
		name   string
		keys   []string
		remove string
		update string
	}{
		{"root leaf", []string{"a", "b"}, "a", "b"},
		{"root branch", []string{"aa", "ab", "b"}, "b", "aa"},
		{"nested leaf", []string{"xaa", "xab", "y"}, "xaa", "xab"},
		{"nested branch", []string{"xaa", "xab", "xb", "y"}, "xb", "xaa"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tr, m := New[int](), trieModel{}
			for _, k := range tt.keys {
				tr.Upsert([]byte(k), 1)
				m[k] = 1
			}
			tx := tr.Txn()
			if old, found := tx.Delete([]byte(tt.remove)); !found || old != 1 {
				t.Fatalf("Delete(%q) = %d, %v; want 1, true", tt.remove, old, found)
			}
			checkCowFrontier(t, "after collapse", tx.NewTrie(), tr)
			if old, found := tx.Upsert([]byte(tt.update), 2); !found || old != 1 {
				t.Fatalf("Upsert(%q) = %d, %v; want 1, true", tt.update, old, found)
			}
			nm := m.clone()
			delete(nm, tt.remove)
			nm[tt.update] = 2
			checkModel(t, "committed", tx.Commit(), nm, nil)
			checkModel(t, "original", tr, m, nil)
		})
	}
}

func checkCowGenericValues[V any](t *testing.T, a, b V) {
	t.Helper()
	check := func(tr *Trie[V], key string, want V) {
		t.Helper()
		if got, found := tr.Get([]byte(key)); !found || !reflect.DeepEqual(got, want) {
			t.Fatalf("Get(%q) = %v, %v; want %v, true", key, got, found, want)
		}
	}
	tr := New[V]()
	tr.Upsert([]byte("a"), a)
	tr.Upsert([]byte("ab"), a)
	tx := tr.Txn()
	tx.Upsert([]byte("a"), b)
	tx.Delete([]byte("ab"))
	tx.Upsert([]byte("z"), b)
	newer := tx.Commit()
	check(tr, "a", a)
	check(tr, "ab", a)
	check(newer, "a", b)
	check(newer, "z", b)
	tx = tr.Txn()
	tx.Upsert([]byte("ab"), b)
	tx.Abort()
	check(tr, "ab", a)
	if _, found := newer.Get([]byte("ab")); found {
		t.Fatal("deleted value reappeared")
	}
}

func Test_CowGenericValues(t *testing.T) {
	t.Run("pointer", func(t *testing.T) {
		a, b := 1, 2
		checkCowGenericValues(t, &a, &b)
	})
	t.Run("slice", func(t *testing.T) { checkCowGenericValues(t, []byte{1}, []byte{2}) })
	t.Run("map", func(t *testing.T) {
		checkCowGenericValues(t, map[string]int{"v": 1}, map[string]int{"v": 2})
	})
	t.Run("struct", func(t *testing.T) {
		type record struct {
			data [64]byte
			name string
		}
		checkCowGenericValues(t, record{name: "a"}, record{name: "b"})
	})
	t.Run("struct with references", func(t *testing.T) {
		type record struct {
			data   []byte
			labels map[string]int
			count  *int
		}
		a, b := 1, 2
		checkCowGenericValues(t,
			record{data: []byte{1}, labels: map[string]int{"a": 1}, count: &a},
			record{data: []byte{2}, labels: map[string]int{"b": 2}, count: &b},
		)
	})
	t.Run("interface", func(t *testing.T) { checkCowGenericValues[any](t, nil, []int{2}) })
	t.Run("zero size", func(t *testing.T) { checkCowGenericValues(t, struct{}{}, struct{}{}) })
}

func Test_CowReferenceUpdateHandler(t *testing.T) {
	type record struct {
		count int
		tags  []string
	}
	for _, finish := range []string{"commit", "abort"} {
		t.Run(finish, func(t *testing.T) {
			var seen []*record
			tr := New(WithOnUpdate(func(newVal, oldVal *record) *record {
				seen = append(seen, oldVal)
				// Copy the nested slice before appending to keep oldVal immutable.
				tags := append([]string(nil), oldVal.tags...)
				return &record{
					count: oldVal.count + newVal.count,
					tags:  append(tags, newVal.tags...),
				}
			}))
			original := &record{count: 1, tags: []string{"base"}}
			tr.Upsert([]byte("a"), original)
			tr.Upsert([]byte("b"), original)
			tx := tr.Txn()
			if old, updated := tx.Upsert([]byte("a"), &record{count: 2, tags: []string{"one"}}); !updated || old != original {
				t.Fatal("first update did not return the original value")
			}
			first, _ := tx.Get([]byte("a"))
			if old, updated := tx.Upsert([]byte("a"), &record{count: 3, tags: []string{"two"}}); !updated || old != first {
				t.Fatal("second update did not return the previous transaction value")
			}
			if len(seen) != 2 || seen[0] != original || seen[1] != first {
				t.Fatal("handler did not receive the latest value for each update")
			}
			working := tx.NewTrie()
			want := &record{count: 6, tags: []string{"base", "one", "two"}}
			check := func(tr *Trie[*record], key string, want *record) {
				t.Helper()
				if got, found := tr.Get([]byte(key)); !found || !reflect.DeepEqual(got, want) {
					t.Fatalf("Get(%q) = %v, %v; want %v, true", key, got, found, want)
				}
			}
			check(working, "a", want)
			if got, _ := working.Get([]byte("a")); got == original || got == first {
				t.Fatal("handler reused an earlier value")
			}
			if finish == "commit" {
				if got := tx.Commit(); got != working {
					t.Fatal("Commit returned the wrong trie")
				}
			} else if got := tx.Abort(); got != tr {
				t.Fatal("Abort returned the wrong trie")
			}
			check(tr, "a", &record{count: 1, tags: []string{"base"}})
			check(working, "a", want)
			if !reflect.DeepEqual(first, &record{count: 3, tags: []string{"base", "one"}}) {
				t.Fatalf("second update mutated the first result: %v", first)
			}
			for _, version := range []*Trie[*record]{tr, working} {
				if got, found := version.Get([]byte("b")); !found || got != original {
					t.Fatal("unchanged key did not retain its original value")
				}
			}
		})
	}
}

func Test_CowCallbackPanic(t *testing.T) {
	fail := false
	tr := New(WithOnInsert(func(v int) int {
		if fail {
			panic("insert")
		}
		return v
	}), WithOnUpdate(func(v, old int) int {
		if fail {
			panic("update")
		}
		return v
	}))
	for _, k := range []string{"a", "ab", "b", "c"} {
		tr.Upsert([]byte(k), 1)
	}
	m := trieModel{"a": 1, "ab": 1, "b": 1, "c": 1}
	tx := tr.Txn()
	fail = true
	for _, k := range []string{"a", "ax", "z", "b"} {
		func() {
			defer func() {
				want := "insert"
				if _, found := m[k]; found {
					want = "update"
				}
				if got := recover(); got != want {
					t.Fatalf("Upsert(%q) panic = %v, want %q", k, got, want)
				}
			}()
			tx.Upsert([]byte(k), 2)
		}()
		checkModel(t, "after panic base", tr, m, nil)
		checkModel(t, "after panic tx", tx.NewTrie(), m, nil)
	}
	fail = false
	tx.Upsert([]byte("z"), 3)
	nm := m.clone()
	nm["z"] = 3
	checkModel(t, "continued tx", tx.Commit(), nm, nil)
	checkModel(t, "original", tr, m, nil)
}
