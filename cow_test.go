package qp

import (
	"math"
	"reflect"
	"testing"
)

type updateItem struct {
	key            []byte
	val            any
	expectOldVal   any
	expectIsUpdate bool
}

type deleteItem struct {
	key          []byte
	expectOldVal any
	expectFound  bool
}

func Test_CowUpsert(t *testing.T) {
	tests := []struct {
		name        string
		oldTr       []KVPair
		updates     []updateItem
		expectOldTr []KVPair
		expectNewTr []KVPair
	}{
		{
			name:  "Empty cow",
			oldTr: []KVPair{},
			updates: []updateItem{
				{[]byte("a"), value1, nil, false},
				{[]byte("b"), value1, nil, false},
			},
			expectOldTr: nil,
			expectNewTr: []KVPair{
				{[]byte("a"), value1},
				{[]byte("b"), value1},
			},
		},
		{
			name: "Simple cow",
			oldTr: []KVPair{
				{[]byte("a"), value1},
				{[]byte("b"), value1},
				{[]byte("c"), value1},
				{[]byte("d"), value1},
			},
			updates: []updateItem{
				{[]byte("b"), value2, value1, true},
				{[]byte("c"), value2, value1, true},
				{[]byte("e"), value2, nil, false},
			},
			expectOldTr: []KVPair{
				{[]byte("a"), value1},
				{[]byte("b"), value1},
				{[]byte("c"), value1},
				{[]byte("d"), value1},
			},
			expectNewTr: []KVPair{
				{[]byte("a"), value1},
				{[]byte("b"), value2},
				{[]byte("c"), value2},
				{[]byte("d"), value1},
				{[]byte("e"), value2},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tr := New()
			for _, kv := range tt.oldTr {
				upsert(t, tr, kv.Key, kv.Value)
			}

			tx := begin(t, tr)
			for _, update := range tt.updates {
				oldVal, isUpdate := txUpsert(t, tx, update.key, update.val)
				if isUpdate != update.expectIsUpdate {
					t.Errorf("Upsert(%q) got %v want %v", update.key, isUpdate, update.expectIsUpdate)
				}
				if !reflect.DeepEqual(oldVal, update.expectOldVal) {
					t.Errorf("Upsert(%q) got %v want %v", update.key, oldVal, update.expectOldVal)
				}
			}

			resultOld := tx.oldTr.Walk(math.MaxInt, nil)
			resultNew := tx.newTr.Walk(math.MaxInt, nil)

			if !reflect.DeepEqual(resultOld, tt.expectOldTr) {
				t.Errorf("oldTr.Walk() got %v want %v", resultOld, tt.expectOldTr)
			}
			if !reflect.DeepEqual(resultNew, tt.expectNewTr) {
				t.Errorf("newTr.Walk() got %v want %v", resultNew, tt.expectNewTr)
			}
			if tx.oldTr.Size() != len(resultOld) {
				t.Errorf("oldTr.Size got %v want %v", tx.oldTr.Size(), len(resultOld))
			}
			if tx.newTr.Size() != len(resultNew) {
				t.Errorf("newTr.Size got %v want %v", tx.newTr.Size(), len(resultNew))
			}
		})
	}
}

func Test_CowDelete(t *testing.T) {
	tests := []struct {
		name        string
		oldTr       []KVPair
		deletes     []deleteItem
		expectOldTr []KVPair
		expectNewTr []KVPair
	}{
		{
			name:  "Empty cow",
			oldTr: []KVPair{},
			deletes: []deleteItem{
				{[]byte("a"), nil, false},
				{[]byte("b"), nil, false},
			},
			expectOldTr: nil,
			expectNewTr: nil,
		},
		{
			name: "Simple cow",
			oldTr: []KVPair{
				{[]byte("a"), value1},
				{[]byte("b"), value1},
				{[]byte("c"), value1},
				{[]byte("d"), value1},
			},
			deletes: []deleteItem{
				{[]byte("b"), value1, true},
				{[]byte("c"), value1, true},
				{[]byte("e"), nil, false},
			},
			expectOldTr: []KVPair{
				{[]byte("a"), value1},
				{[]byte("b"), value1},
				{[]byte("c"), value1},
				{[]byte("d"), value1},
			},
			expectNewTr: []KVPair{
				{[]byte("a"), value1},
				{[]byte("d"), value1},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tr := New()
			for _, kv := range tt.oldTr {
				upsert(t, tr, kv.Key, kv.Value)
			}

			tx := begin(t, tr)
			for _, del := range tt.deletes {
				oldVal, found := txDel(t, tx, del.key)
				if found != del.expectFound {
					t.Errorf("Delete(%q) got %v want %v", del.key, found, del.expectFound)
				}
				if !reflect.DeepEqual(oldVal, del.expectOldVal) {
					t.Errorf("Delete(%q) got %v want %v", del.key, oldVal, del.expectOldVal)
				}
			}

			resultOld := tx.oldTr.Walk(math.MaxInt, nil)
			resultNew := tx.newTr.Walk(math.MaxInt, nil)

			if !reflect.DeepEqual(resultOld, tt.expectOldTr) {
				t.Errorf("oldTr.Walk() got %v want %v", resultOld, tt.expectOldTr)
			}
			if !reflect.DeepEqual(resultNew, tt.expectNewTr) {
				t.Errorf("newTr.Walk() got %v want %v", resultNew, tt.expectNewTr)
			}
			if tx.oldTr.Size() != len(resultOld) {
				t.Errorf("oldTr.Size got %v want %v", tx.oldTr.Size(), len(resultOld))
			}
			if tx.newTr.Size() != len(resultNew) {
				t.Errorf("newTr.Size got %v want %v", tx.newTr.Size(), len(resultNew))
			}
		})
	}
}

func Test_CowTxnAfterRemoveTwig(t *testing.T) {
	tr := New()
	upsert(t, tr, []byte("a"), value1)
	upsert(t, tr, []byte("b"), value2)
	upsert(t, tr, []byte("c"), value1)

	del(t, tr, []byte("b"))

	tx := begin(t, tr)
	txUpsert(t, tx, []byte("d"), value2)
	tr = tx.Commit()

	result := tr.Walk(math.MaxInt, nil)
	expect := []KVPair{
		{[]byte("a"), value1},
		{[]byte("c"), value1},
		{[]byte("d"), value2},
	}
	if !reflect.DeepEqual(result, expect) {
		t.Errorf("Walk got %v want %v", result, expect)
	}

	tx2 := begin(t, tr)
	txUpsert(t, tx2, []byte("e"), value1)
	tr = tx2.Commit()

	result = tr.Walk(math.MaxInt, nil)
	expect = append(expect, KVPair{[]byte("e"), value1})
	if !reflect.DeepEqual(result, expect) {
		t.Errorf("Walk after second txn got %v want %v", result, expect)
	}
}

func Test_CowCommitDeleteThenNewTxn(t *testing.T) {
	tr := New()
	upsert(t, tr, []byte("a"), value1)
	upsert(t, tr, []byte("b"), value2)
	upsert(t, tr, []byte("c"), value1)

	tx := begin(t, tr)
	txDel(t, tx, []byte("b"))
	tr = tx.Commit()

	tx2 := begin(t, tr)
	txUpsert(t, tx2, []byte("d"), value2)
	tr = tx2.Abort()

	result := tr.Walk(math.MaxInt, nil)
	expect := []KVPair{
		{[]byte("a"), value1},
		{[]byte("c"), value1},
	}
	if !reflect.DeepEqual(result, expect) {
		t.Errorf("Walk got %v want %v", result, expect)
	}
}

func Test_CowGrowTwigs(t *testing.T) {
	tr := New()
	upsert(t, tr, []byte("ab"), value1)
	upsert(t, tr, []byte("ac"), value2)

	tx := begin(t, tr)
	txUpsert(t, tx, []byte("ad"), value1)

	resultNew := tx.newTr.Walk(math.MaxInt, nil)
	expectNew := []KVPair{
		{[]byte("ab"), value1},
		{[]byte("ac"), value2},
		{[]byte("ad"), value1},
	}
	if !reflect.DeepEqual(resultNew, expectNew) {
		t.Errorf("newTr.Walk got %v want %v", resultNew, expectNew)
	}

	resultOld := tx.oldTr.Walk(math.MaxInt, nil)
	expectOld := []KVPair{
		{[]byte("ab"), value1},
		{[]byte("ac"), value2},
	}
	if !reflect.DeepEqual(resultOld, expectOld) {
		t.Errorf("oldTr.Walk got %v want %v", resultOld, expectOld)
	}
}

func Test_CowDeleteCollapse(t *testing.T) {
	tr := New()
	upsert(t, tr, []byte("a"), value1)
	upsert(t, tr, []byte("b"), value2)

	tx := begin(t, tr)
	oldVal, found := txDel(t, tx, []byte("a"))
	if !found || oldVal != value1 {
		t.Fatalf("Delete(a) got %v %v want %v true", oldVal, found, value1)
	}

	resultNew := tx.newTr.Walk(math.MaxInt, nil)
	expectNew := []KVPair{{[]byte("b"), value2}}
	if !reflect.DeepEqual(resultNew, expectNew) {
		t.Errorf("newTr.Walk got %v want %v", resultNew, expectNew)
	}

	resultOld := tx.oldTr.Walk(math.MaxInt, nil)
	expectOld := []KVPair{
		{[]byte("a"), value1},
		{[]byte("b"), value2},
	}
	if !reflect.DeepEqual(resultOld, expectOld) {
		t.Errorf("oldTr.Walk got %v want %v", resultOld, expectOld)
	}

	tr = tx.Commit()
	result := tr.Walk(math.MaxInt, nil)
	if !reflect.DeepEqual(result, expectNew) {
		t.Errorf("Walk after commit got %v want %v", result, expectNew)
	}
}

func Test_CowDeleteThenUpsertInTxn(t *testing.T) {
	tr := New()
	upsert(t, tr, []byte("a"), value1)
	upsert(t, tr, []byte("b"), value2)
	upsert(t, tr, []byte("c"), value1)

	tx := begin(t, tr)
	if _, found := txDel(t, tx, []byte("b")); !found {
		t.Fatal("Delete(b) not found")
	}
	txUpsert(t, tx, []byte("d"), value2)

	resultNew := tx.newTr.Walk(math.MaxInt, nil)
	expectNew := []KVPair{
		{[]byte("a"), value1},
		{[]byte("c"), value1},
		{[]byte("d"), value2},
	}
	if !reflect.DeepEqual(resultNew, expectNew) {
		t.Errorf("newTr.Walk got %v want %v", resultNew, expectNew)
	}

	resultOld := tx.oldTr.Walk(math.MaxInt, nil)
	expectOld := []KVPair{
		{[]byte("a"), value1},
		{[]byte("b"), value2},
		{[]byte("c"), value1},
	}
	if !reflect.DeepEqual(resultOld, expectOld) {
		t.Errorf("oldTr.Walk got %v want %v", resultOld, expectOld)
	}
}

func Test_CowOnInsert(t *testing.T) {
	onInsert := func(newVal any) any {
		return newVal.(int) * 10
	}
	tr := New(WithOnInsert(onInsert))

	tx := begin(t, tr)
	txUpsert(t, tx, []byte("a"), 5)

	val, found := txGet(t, tx, []byte("a"))
	if !found || val.(int) != 50 {
		t.Errorf("Get(a) got %v found=%v want 50 true", val, found)
	}

	tr = tx.Commit()
	val, found = get(t, tr, []byte("a"))
	if !found || val.(int) != 50 {
		t.Errorf("after commit Get(a) got %v found=%v want 50 true", val, found)
	}
}

func Test_CowOnUpdate(t *testing.T) {
	onUpdate := func(newVal, oldVal any) any {
		return newVal.(int) + oldVal.(int)
	}
	tr := New(WithOnUpdate(onUpdate))
	upsert(t, tr, []byte("a"), 1)

	tx := begin(t, tr)
	oldVal, isUpdate := txUpsert(t, tx, []byte("a"), 2)
	if !isUpdate || oldVal.(int) != 1 {
		t.Fatalf("Upsert(a) got oldVal=%v isUpdate=%v want 1 true", oldVal, isUpdate)
	}

	val, found := txGet(t, tx, []byte("a"))
	if !found || val.(int) != 3 {
		t.Errorf("Get(a) got %v found=%v want 3 true", val, found)
	}

	resultOld := tx.oldTr.Walk(math.MaxInt, nil)
	expectOld := []KVPair{{[]byte("a"), 1}}
	if !reflect.DeepEqual(resultOld, expectOld) {
		t.Errorf("oldTr.Walk got %v want %v", resultOld, expectOld)
	}
}

func Test_CowCommit(t *testing.T) {
	tests := []struct {
		name    string
		oldTr   []KVPair
		updates []updateItem
		deletes []deleteItem
		expectR []KVPair
	}{
		{
			name:  "Empty cow",
			oldTr: []KVPair{},
			updates: []updateItem{
				{[]byte("a"), value1, nil, false},
				{[]byte("b"), value1, nil, false},
			},
			deletes: []deleteItem{
				{[]byte("b"), value1, true},
				{[]byte("c"), nil, false},
			},
			expectR: []KVPair{
				{[]byte("a"), value1},
			},
		},
		{
			name: "Simple cow",
			oldTr: []KVPair{
				{[]byte("a"), value1},
				{[]byte("b"), value1},
				{[]byte("c"), value1},
				{[]byte("d"), value1},
			},
			updates: []updateItem{
				{[]byte("a"), value2, value1, true},
				{[]byte("b"), value2, value1, true},
			},
			deletes: []deleteItem{
				{[]byte("b"), value2, true},
				{[]byte("c"), value1, true},
				{[]byte("e"), nil, false},
			},
			expectR: []KVPair{
				{[]byte("a"), value2},
				{[]byte("d"), value1},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tr := New()
			for _, kv := range tt.oldTr {
				upsert(t, tr, kv.Key, kv.Value)
			}

			tx := begin(t, tr)
			for _, update := range tt.updates {
				oldVal, isUpdate := txUpsert(t, tx, update.key, update.val)
				if isUpdate != update.expectIsUpdate {
					t.Errorf("Upsert(%q) got %v want %v", update.key, isUpdate, update.expectIsUpdate)
				}
				if !reflect.DeepEqual(oldVal, update.expectOldVal) {
					t.Errorf("Upsert(%q) got %v want %v", update.key, oldVal, update.expectOldVal)
				}
			}
			for _, del := range tt.deletes {
				oldVal, found := txDel(t, tx, del.key)
				if found != del.expectFound {
					t.Errorf("Delete(%q) got %v want %v", del.key, found, del.expectFound)
				}
				if !reflect.DeepEqual(oldVal, del.expectOldVal) {
					t.Errorf("Delete(%q) got %v want %v", del.key, oldVal, del.expectOldVal)
				}
			}

			tr = tx.Commit()
			result := tr.Walk(math.MaxInt, nil)

			if !reflect.DeepEqual(result, tt.expectR) {
				t.Errorf("Walk got %v want %v", result, tt.expectR)
			}
			if tr.Size() != len(result) {
				t.Errorf("Size got %v want %v", tr.Size(), len(result))
			}
		})
	}
}

func Test_CowAbort(t *testing.T) {
	tests := []struct {
		name    string
		oldTr   []KVPair
		updates []updateItem
		deletes []deleteItem
		expectR []KVPair
	}{
		{
			name:  "Empty cow",
			oldTr: []KVPair{},
			updates: []updateItem{
				{[]byte("a"), value1, nil, false},
				{[]byte("b"), value1, nil, false},
			},
			deletes: []deleteItem{
				{[]byte("b"), value1, true},
				{[]byte("c"), nil, false},
			},
			expectR: nil,
		},
		{
			name: "Simple cow",
			oldTr: []KVPair{
				{[]byte("a"), value1},
				{[]byte("b"), value1},
				{[]byte("c"), value1},
				{[]byte("d"), value1},
			},
			updates: []updateItem{
				{[]byte("a"), value2, value1, true},
				{[]byte("b"), value2, value1, true},
			},
			deletes: []deleteItem{
				{[]byte("b"), value2, true},
				{[]byte("c"), value1, true},
				{[]byte("e"), nil, false},
			},
			expectR: []KVPair{
				{[]byte("a"), value1},
				{[]byte("b"), value1},
				{[]byte("c"), value1},
				{[]byte("d"), value1},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tr := New()
			for _, kv := range tt.oldTr {
				upsert(t, tr, kv.Key, kv.Value)
			}

			tx := begin(t, tr)
			for _, update := range tt.updates {
				oldVal, isUpdate := txUpsert(t, tx, update.key, update.val)
				if isUpdate != update.expectIsUpdate {
					t.Errorf("Upsert(%q) got %v want %v", update.key, isUpdate, update.expectIsUpdate)
				}
				if !reflect.DeepEqual(oldVal, update.expectOldVal) {
					t.Errorf("Upsert(%q) got %v want %v", update.key, oldVal, update.expectOldVal)
				}
			}
			for _, del := range tt.deletes {
				oldVal, found := txDel(t, tx, del.key)
				if found != del.expectFound {
					t.Errorf("Delete(%q) got %v want %v", del.key, found, del.expectFound)
				}
				if !reflect.DeepEqual(oldVal, del.expectOldVal) {
					t.Errorf("Delete(%q) got %v want %v", del.key, oldVal, del.expectOldVal)
				}
			}

			tr = tx.Abort()
			result := tr.Walk(math.MaxInt, nil)

			if !reflect.DeepEqual(result, tt.expectR) {
				t.Errorf("Walk got %v want %v", result, tt.expectR)
			}
			if tr.Size() != len(result) {
				t.Errorf("Size got %v want %v", tr.Size(), len(result))
			}
		})
	}
}

func Test_CowIndependentAfterCommit(t *testing.T) {
	base := New()
	keys := []string{"a", "b", "c", "f", "cef", "e", "cefy"}
	for _, k := range keys {
		upsert(t, base, []byte(k), value1)
	}

	tx := begin(t, base)
	txUpsert(t, tx, []byte("zz"), value2)
	committed := tx.Commit()

	upsert(t, committed, []byte("cefx"), value2)
	del(t, committed, []byte("a"))

	if _, ok := get(t, base, []byte("zz")); ok {
		t.Fatal("base saw a key inserted in the committed trie")
	}
	if _, ok := get(t, base, []byte("cefx")); ok {
		t.Fatal("base saw a key inserted after commit")
	}
	if _, ok := get(t, base, []byte("a")); !ok {
		t.Fatal("base lost key a after the committed trie deleted it")
	}
	if base.Size() != len(keys) {
		t.Fatalf("base.Size() = %d, want %d", base.Size(), len(keys))
	}

	upsert(t, base, []byte("d"), value2)
	del(t, base, []byte("b"))

	if _, ok := get(t, committed, []byte("d")); ok {
		t.Fatal("committed trie saw a key inserted in the source trie")
	}
	if _, ok := get(t, committed, []byte("b")); !ok {
		t.Fatal("committed trie lost key b after the source trie deleted it")
	}
	if _, ok := get(t, committed, []byte("a")); ok {
		t.Fatal("committed trie still has key a after Delete")
	}
	if committed.Size() != len(keys)+1 { // +zz +cefx -a
		t.Fatalf("committed.Size() = %d, want %d", committed.Size(), len(keys)+1)
	}

	baseWalk := base.Walk(math.MaxInt, nil)
	expectBase := []KVPair{
		{[]byte("a"), value1},
		{[]byte("c"), value1},
		{[]byte("cef"), value1},
		{[]byte("cefy"), value1},
		{[]byte("d"), value2},
		{[]byte("e"), value1},
		{[]byte("f"), value1},
	}
	if !reflect.DeepEqual(baseWalk, expectBase) {
		t.Errorf("base.Walk got %v want %v", baseWalk, expectBase)
	}

	committedWalk := committed.Walk(math.MaxInt, nil)
	expectCommitted := []KVPair{
		{[]byte("b"), value1},
		{[]byte("c"), value1},
		{[]byte("cef"), value1},
		{[]byte("cefx"), value2},
		{[]byte("cefy"), value1},
		{[]byte("e"), value1},
		{[]byte("f"), value1},
		{[]byte("zz"), value2},
	}
	if !reflect.DeepEqual(committedWalk, expectCommitted) {
		t.Errorf("committed.Walk got %v want %v", committedWalk, expectCommitted)
	}
}

func Test_CowTwoTxnsFromSameBase(t *testing.T) {
	base := New()
	upsert(t, base, []byte("a"), value1)
	upsert(t, base, []byte("b"), value1)
	upsert(t, base, []byte("c"), value1)

	tx1 := begin(t, base)
	txUpsert(t, tx1, []byte("a"), value2)
	txDel(t, tx1, []byte("c"))
	tr1 := tx1.Commit()

	tx2 := begin(t, base)
	txUpsert(t, tx2, []byte("a"), 3)
	txDel(t, tx2, []byte("b"))
	tr2 := tx2.Commit()

	if v, ok := get(t, base, []byte("a")); !ok || v != value1 {
		t.Fatalf("base Get(a) = (%v, %v), want (%d, true)", v, ok, value1)
	}
	if _, ok := get(t, base, []byte("c")); !ok {
		t.Fatal("base lost key c")
	}
	if v, ok := get(t, tr1, []byte("a")); !ok || v != value2 {
		t.Fatalf("tr1 Get(a) = (%v, %v), want (%d, true)", v, ok, value2)
	}
	if _, ok := get(t, tr1, []byte("c")); ok {
		t.Fatal("tr1 still has key c")
	}
	if _, ok := get(t, tr1, []byte("b")); !ok {
		t.Fatal("tr1 lost key b")
	}
	if v, ok := get(t, tr2, []byte("a")); !ok || v != 3 {
		t.Fatalf("tr2 Get(a) = (%v, %v), want (3, true)", v, ok)
	}
	if _, ok := get(t, tr2, []byte("b")); ok {
		t.Fatal("tr2 still has key b")
	}
	if _, ok := get(t, tr2, []byte("c")); !ok {
		t.Fatal("tr2 lost key c")
	}
}

func Test_CowCommitTwice(t *testing.T) {
	tr := New()
	upsert(t, tr, []byte("a"), value1)

	tx := begin(t, tr)
	txUpsert(t, tx, []byte("b"), value2)
	tr1 := tx.Commit()
	tr2 := tx.Commit()
	if tr1 != tr2 {
		t.Fatal("second Commit returned a different trie")
	}
	if tr2.Size() != 2 {
		t.Fatalf("Size = %d, want 2", tr2.Size())
	}
	if v, ok := get(t, tr2, []byte("b")); !ok || v != value2 {
		t.Fatalf("Get(b) = (%v, %v), want (%d, true)", v, ok, value2)
	}
}

func Test_CowTxnFinished(t *testing.T) {
	tr := New()
	upsert(t, tr, []byte("a"), value1)

	tx := begin(t, tr)
	tx.Commit()
	assertTxnFinished(t, tx)

	tx = begin(t, tr)
	tx.Abort()
	assertTxnFinished(t, tx)
}

func assertTxnFinished(t *testing.T, tx *Txn) {
	t.Helper()
	if _, _, err := tx.Get([]byte("a")); err != ErrTxnFinished {
		t.Fatalf("Get err=%v want %v", err, ErrTxnFinished)
	}
	if _, _, err := tx.Upsert([]byte("b"), value2); err != ErrTxnFinished {
		t.Fatalf("Upsert err=%v want %v", err, ErrTxnFinished)
	}
	if _, _, err := tx.Delete([]byte("a")); err != ErrTxnFinished {
		t.Fatalf("Delete err=%v want %v", err, ErrTxnFinished)
	}
	if _, err := tx.NewTrie(); err != ErrTxnFinished {
		t.Fatalf("NewTrie err=%v want %v", err, ErrTxnFinished)
	}
}

func Test_CowTxnCopiesKey(t *testing.T) {
	tr := New()
	tx := begin(t, tr)
	buf := []byte("key0")
	txUpsert(t, tx, buf, value1)
	buf[3] = '1'
	txUpsert(t, tx, buf, value2)
	tr = tx.Commit()

	if tr.Size() != 2 {
		t.Fatalf("size = %d, want 2", tr.Size())
	}
	if v, ok := get(t, tr, []byte("key0")); !ok || v != value1 {
		t.Fatalf("Get(key0) = (%v, %v), want (%d, true)", v, ok, value1)
	}
	if v, ok := get(t, tr, []byte("key1")); !ok || v != value2 {
		t.Fatalf("Get(key1) = (%v, %v), want (%d, true)", v, ok, value2)
	}
}

func Test_DeleteMissDoesNotCow(t *testing.T) {
	tr := New()
	upsert(t, tr, []byte("a"), value1)
	upsert(t, tr, []byte("b"), value1)

	tx := begin(t, tr)
	root := tx.newTr.root
	if _, found := txDel(t, tx, []byte("zzz")); found {
		t.Fatal("Delete miss reported found")
	}
	if tx.newTr.root != root {
		t.Fatal("Delete miss copied the root")
	}
	if tx.oldTr.root != tx.newTr.root {
		t.Fatal("Delete miss split the shared root")
	}
}

func Test_TxnAlreadyOpen(t *testing.T) {
	tr := New()
	upsert(t, tr, []byte("a"), value1)

	tx := begin(t, tr)
	if _, err := tr.Txn(); err != ErrTxnOpen {
		t.Fatalf("origin Txn err=%v want %v", err, ErrTxnOpen)
	}
	if _, err := tx.OldTrie().Txn(); err != ErrTxnOpen {
		t.Fatalf("OldTrie Txn err=%v want %v", err, ErrTxnOpen)
	}
	work, err := tx.NewTrie()
	if err != nil {
		t.Fatal(err)
	}
	if _, err := work.Txn(); err != ErrTxnOpen {
		t.Fatalf("NewTrie Txn err=%v want %v", err, ErrTxnOpen)
	}
	if _, _, err := tr.Upsert([]byte("b"), value2); err != ErrTrieFrozen {
		t.Fatalf("Upsert err=%v want %v", err, ErrTrieFrozen)
	}
	if _, _, err := tr.Delete([]byte("a")); err != ErrTrieFrozen {
		t.Fatalf("Delete err=%v want %v", err, ErrTrieFrozen)
	}

	if v, ok := get(t, tr, []byte("a")); !ok || v != value1 {
		t.Fatalf("Get on origin during txn: (%v, %v)", v, ok)
	}

	txUpsert(t, tx, []byte("b"), value2)
	tr = tx.Commit()
	if v, ok := get(t, tr, []byte("b")); !ok || v != value2 {
		t.Fatalf("Get after commit: (%v, %v)", v, ok)
	}

	tx2 := begin(t, tr)
	tx2.Abort()
	upsert(t, tr, []byte("c"), value1)
	if _, err := tr.Txn(); err != nil {
		t.Fatal(err)
	}
}

func Test_FamilyTxnFreeze(t *testing.T) {
	origin := New()
	upsert(t, origin, []byte("a"), value1)

	tx := begin(t, origin)
	txUpsert(t, tx, []byte("b"), value2)
	committed := tx.Commit()

	upsert(t, origin, []byte("c"), value1)
	upsert(t, committed, []byte("d"), value2)

	tx2 := begin(t, committed)
	if _, err := origin.Txn(); err != ErrTxnOpen {
		t.Fatalf("origin Txn during descendant txn err=%v want %v", err, ErrTxnOpen)
	}
	if _, _, err := origin.Upsert([]byte("e"), value1); err != ErrTrieFrozen {
		t.Fatalf("origin Upsert during descendant txn err=%v want %v", err, ErrTrieFrozen)
	}
	if _, _, err := origin.Delete([]byte("a")); err != ErrTrieFrozen {
		t.Fatalf("origin Delete during descendant txn err=%v want %v", err, ErrTrieFrozen)
	}
	if _, ok := get(t, origin, []byte("a")); !ok {
		t.Fatal("origin Get during descendant txn failed")
	}
	tx2.Abort()

	upsert(t, origin, []byte("e"), value1)

	txB := begin(t, origin)
	snapB := txB.Commit()
	txC := begin(t, origin)
	snapC := txC.Commit()

	tx3 := begin(t, snapC)
	if _, _, err := snapB.Upsert([]byte("f"), value1); err != ErrTrieFrozen {
		t.Fatalf("sibling snapshot write err=%v want %v", err, ErrTrieFrozen)
	}
	if _, err := snapB.Txn(); err != ErrTxnOpen {
		t.Fatalf("sibling Txn err=%v want %v", err, ErrTxnOpen)
	}
	tx3.Abort()

	other := New()
	tx4 := begin(t, other)
	upsert(t, origin, []byte("g"), value1)
	tx4.Abort()
}
