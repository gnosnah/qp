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
				t.Errorf("newTr.Size got %v want %v", tx.oldTr.Size(), len(resultOld))
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
				t.Errorf("newTr.Size got %v want %v", tx.oldTr.Size(), len(resultOld))
			}
		})
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
