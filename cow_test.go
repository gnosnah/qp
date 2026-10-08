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
				tr.Upsert(kv.Key, kv.Value)
			}

			tx := tr.Txn()
			for _, update := range tt.updates {
				oldVal, isUpdate := tx.Upsert(update.key, update.val)
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
				tr.Upsert(kv.Key, kv.Value)
			}

			tx := tr.Txn()
			for _, del := range tt.deletes {
				oldVal, found := tx.Delete(del.key)
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

func Test_CowRemoveTwigShrinksSlice(t *testing.T) {
	keys := make([][]byte, 4)
	for i := range keys {
		keys[i] = []byte{0x60 + byte(i)}
	}

	tr := New()
	for _, key := range keys {
		tr.Upsert(key, value1)
	}

	tx := tr.Txn()
	if _, found := tx.Delete(keys[1]); !found {
		t.Fatal("Delete not found")
	}

	oldBn := tx.oldTr.root.(*branchNode)
	newBn := tx.newTr.root.(*branchNode)
	if len(oldBn.twigs) != 4 || oldBn.twigOffsetMax() != 4 {
		t.Fatalf("old twigs len=%d live=%d, want 4", len(oldBn.twigs), oldBn.twigOffsetMax())
	}
	assertTwigsTight(t, tx.newTr.root, 3)
	if &oldBn.twigs[0] == &newBn.twigs[0] {
		t.Fatal("txn delete mutated the original twigs slice")
	}

	const cycles = 1000
	for i := range cycles {
		tx.Upsert(keys[1], value2)
		if _, found := tx.Delete(keys[1]); !found {
			t.Fatalf("cycle %d: Delete not found", i)
		}
	}
	assertTwigsTight(t, tx.newTr.root, 3)
	if len(oldBn.twigs) != 4 {
		t.Fatalf("old twigs len=%d after churn, want 4", len(oldBn.twigs))
	}

	resultOld := tx.oldTr.Walk(math.MaxInt, nil)
	if len(resultOld) != 4 {
		t.Fatalf("oldTr.Walk len=%d, want 4", len(resultOld))
	}
	if _, found := tx.Get(keys[1]); found {
		t.Fatal("deleted key still present in txn")
	}
	for _, idx := range []int{0, 2, 3} {
		if _, found := tx.Get(keys[idx]); !found {
			t.Fatalf("Get(%x) missing", keys[idx])
		}
	}
}

func Test_CowTxnAfterRemoveTwig(t *testing.T) {
	tr := New()
	tr.Upsert([]byte("a"), value1)
	tr.Upsert([]byte("b"), value2)
	tr.Upsert([]byte("c"), value1)

	tr.Delete([]byte("b"))

	tx := tr.Txn()
	tx.Upsert([]byte("d"), value2)
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

	tx2 := tr.Txn()
	tx2.Upsert([]byte("e"), value1)
	tr = tx2.Commit()

	result = tr.Walk(math.MaxInt, nil)
	expect = append(expect, KVPair{[]byte("e"), value1})
	if !reflect.DeepEqual(result, expect) {
		t.Errorf("Walk after second txn got %v want %v", result, expect)
	}
}

func Test_CowCommitDeleteThenNewTxn(t *testing.T) {
	tr := New()
	tr.Upsert([]byte("a"), value1)
	tr.Upsert([]byte("b"), value2)
	tr.Upsert([]byte("c"), value1)

	tx := tr.Txn()
	tx.Delete([]byte("b"))
	tr = tx.Commit()

	tx2 := tr.Txn()
	tx2.Upsert([]byte("d"), value2)
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
	tr.Upsert([]byte("ab"), value1)
	tr.Upsert([]byte("ac"), value2)

	tx := tr.Txn()
	tx.Upsert([]byte("ad"), value1)

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
	tr.Upsert([]byte("a"), value1)
	tr.Upsert([]byte("b"), value2)

	tx := tr.Txn()
	oldVal, found := tx.Delete([]byte("a"))
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
	tr.Upsert([]byte("a"), value1)
	tr.Upsert([]byte("b"), value2)
	tr.Upsert([]byte("c"), value1)

	tx := tr.Txn()
	if _, found := tx.Delete([]byte("b")); !found {
		t.Fatal("Delete(b) not found")
	}
	tx.Upsert([]byte("d"), value2)

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

	tx := tr.Txn()
	tx.Upsert([]byte("a"), 5)

	val, found := tx.Get([]byte("a"))
	if !found || val.(int) != 50 {
		t.Errorf("Get(a) got %v found=%v want 50 true", val, found)
	}

	tr = tx.Commit()
	val, found = tr.Get([]byte("a"))
	if !found || val.(int) != 50 {
		t.Errorf("after commit Get(a) got %v found=%v want 50 true", val, found)
	}
}

func Test_CowOnUpdate(t *testing.T) {
	onUpdate := func(newVal, oldVal any) any {
		return newVal.(int) + oldVal.(int)
	}
	tr := New(WithOnUpdate(onUpdate))
	tr.Upsert([]byte("a"), 1)

	tx := tr.Txn()
	oldVal, isUpdate := tx.Upsert([]byte("a"), 2)
	if !isUpdate || oldVal.(int) != 1 {
		t.Fatalf("Upsert(a) got oldVal=%v isUpdate=%v want 1 true", oldVal, isUpdate)
	}

	val, found := tx.Get([]byte("a"))
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
				tr.Upsert(kv.Key, kv.Value)
			}

			tx := tr.Txn()
			for _, update := range tt.updates {
				oldVal, isUpdate := tx.Upsert(update.key, update.val)
				if isUpdate != update.expectIsUpdate {
					t.Errorf("Upsert(%q) got %v want %v", update.key, isUpdate, update.expectIsUpdate)
				}
				if !reflect.DeepEqual(oldVal, update.expectOldVal) {
					t.Errorf("Upsert(%q) got %v want %v", update.key, oldVal, update.expectOldVal)
				}
			}
			for _, del := range tt.deletes {
				oldVal, found := tx.Delete(del.key)
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
				tr.Upsert(kv.Key, kv.Value)
			}

			tx := tr.Txn()
			for _, update := range tt.updates {
				oldVal, isUpdate := tx.Upsert(update.key, update.val)
				if isUpdate != update.expectIsUpdate {
					t.Errorf("Upsert(%q) got %v want %v", update.key, isUpdate, update.expectIsUpdate)
				}
				if !reflect.DeepEqual(oldVal, update.expectOldVal) {
					t.Errorf("Upsert(%q) got %v want %v", update.key, oldVal, update.expectOldVal)
				}
			}
			for _, del := range tt.deletes {
				oldVal, found := tx.Delete(del.key)
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
