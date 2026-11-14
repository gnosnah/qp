package qp

import (
	"bytes"
	"math"
	"math/rand"
	"reflect"
	"sync"
	"sync/atomic"
	"testing"
)

type updateItem struct {
	key            []byte
	val            int
	expectOldVal   int
	expectIsUpdate bool
}

type deleteItem struct {
	key          []byte
	expectOldVal int
	expectFound  bool
}

func Test_CowUpsert(t *testing.T) {
	tests := []struct {
		name        string
		oldTr       []KVPair[int]
		updates     []updateItem
		expectOldTr []KVPair[int]
		expectNewTr []KVPair[int]
	}{
		{
			name:  "Empty cow",
			oldTr: []KVPair[int]{},
			updates: []updateItem{
				{[]byte("a"), value1, 0, false},
				{[]byte("b"), value1, 0, false},
			},
			expectOldTr: nil,
			expectNewTr: []KVPair[int]{
				{[]byte("a"), value1},
				{[]byte("b"), value1},
			},
		},
		{
			name: "Simple cow",
			oldTr: []KVPair[int]{
				{[]byte("a"), value1},
				{[]byte("b"), value1},
				{[]byte("c"), value1},
				{[]byte("d"), value1},
			},
			updates: []updateItem{
				{[]byte("b"), value2, value1, true},
				{[]byte("c"), value2, value1, true},
				{[]byte("e"), value2, 0, false},
			},
			expectOldTr: []KVPair[int]{
				{[]byte("a"), value1},
				{[]byte("b"), value1},
				{[]byte("c"), value1},
				{[]byte("d"), value1},
			},
			expectNewTr: []KVPair[int]{
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
			tr := New[int]()
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

func Test_CowUpsertCopiesKey(t *testing.T) {
	buf := []byte("alpha")
	tx := New[int]().Txn()
	tx.Upsert(buf, value1)
	if aliasesCaller(tx.newTr.root, []byte("alpha"), buf) {
		t.Fatal("txn root leaf aliases the caller buffer")
	}
	copy(buf, "brave")
	tx.Upsert(buf, value2)
	if aliasesCaller(tx.newTr.root, []byte("brave"), buf) {
		t.Fatal("txn leaf aliases the caller buffer")
	}
	copy(buf, "xxxxx")

	tr := tx.Commit()
	got, found := tr.Get([]byte("alpha"))
	if !found || got != value1 {
		t.Fatalf("Get(alpha) = %v found=%v, want %d true", got, found, value1)
	}
	got, found = tr.Get([]byte("brave"))
	if !found || got != value2 {
		t.Fatalf("Get(brave) = %v found=%v, want %d true", got, found, value2)
	}
	if _, found = tr.Get([]byte("xxxxx")); found {
		t.Fatal("mutated buffer became a key")
	}
}

func Test_CowDelete(t *testing.T) {
	tests := []struct {
		name        string
		oldTr       []KVPair[int]
		deletes     []deleteItem
		expectOldTr []KVPair[int]
		expectNewTr []KVPair[int]
	}{
		{
			name:  "Empty cow",
			oldTr: []KVPair[int]{},
			deletes: []deleteItem{
				{[]byte("a"), 0, false},
				{[]byte("b"), 0, false},
			},
			expectOldTr: nil,
			expectNewTr: nil,
		},
		{
			name: "Simple cow",
			oldTr: []KVPair[int]{
				{[]byte("a"), value1},
				{[]byte("b"), value1},
				{[]byte("c"), value1},
				{[]byte("d"), value1},
			},
			deletes: []deleteItem{
				{[]byte("b"), value1, true},
				{[]byte("c"), value1, true},
				{[]byte("e"), 0, false},
			},
			expectOldTr: []KVPair[int]{
				{[]byte("a"), value1},
				{[]byte("b"), value1},
				{[]byte("c"), value1},
				{[]byte("d"), value1},
			},
			expectNewTr: []KVPair[int]{
				{[]byte("a"), value1},
				{[]byte("d"), value1},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tr := New[int]()
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

	tr := New[int]()
	for _, key := range keys {
		tr.Upsert(key, value1)
	}

	tx := tr.Txn()
	if _, found := tx.Delete(keys[1]); !found {
		t.Fatal("Delete not found")
	}

	oldBn := tx.oldTr.root.(*branchNode[int])
	newBn := tx.newTr.root.(*branchNode[int])
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
	tr := New[int]()
	tr.Upsert([]byte("a"), value1)
	tr.Upsert([]byte("b"), value2)
	tr.Upsert([]byte("c"), value1)

	tr.Delete([]byte("b"))

	tx := tr.Txn()
	tx.Upsert([]byte("d"), value2)
	tr = tx.Commit()

	result := tr.Walk(math.MaxInt, nil)
	expect := []KVPair[int]{
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
	expect = append(expect, KVPair[int]{[]byte("e"), value1})
	if !reflect.DeepEqual(result, expect) {
		t.Errorf("Walk after second txn got %v want %v", result, expect)
	}
}

func Test_CowCommitDeleteThenNewTxn(t *testing.T) {
	tr := New[int]()
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
	expect := []KVPair[int]{
		{[]byte("a"), value1},
		{[]byte("c"), value1},
	}
	if !reflect.DeepEqual(result, expect) {
		t.Errorf("Walk got %v want %v", result, expect)
	}
}

func Test_CowGrowTwigs(t *testing.T) {
	tr := New[int]()
	tr.Upsert([]byte("ab"), value1)
	tr.Upsert([]byte("ac"), value2)

	tx := tr.Txn()
	tx.Upsert([]byte("ad"), value1)

	resultNew := tx.newTr.Walk(math.MaxInt, nil)
	expectNew := []KVPair[int]{
		{[]byte("ab"), value1},
		{[]byte("ac"), value2},
		{[]byte("ad"), value1},
	}
	if !reflect.DeepEqual(resultNew, expectNew) {
		t.Errorf("newTr.Walk got %v want %v", resultNew, expectNew)
	}

	resultOld := tx.oldTr.Walk(math.MaxInt, nil)
	expectOld := []KVPair[int]{
		{[]byte("ab"), value1},
		{[]byte("ac"), value2},
	}
	if !reflect.DeepEqual(resultOld, expectOld) {
		t.Errorf("oldTr.Walk got %v want %v", resultOld, expectOld)
	}
}

func Test_CowDeleteCollapse(t *testing.T) {
	tr := New[int]()
	tr.Upsert([]byte("a"), value1)
	tr.Upsert([]byte("b"), value2)

	tx := tr.Txn()
	oldVal, found := tx.Delete([]byte("a"))
	if !found || oldVal != value1 {
		t.Fatalf("Delete(a) got %v %v want %v true", oldVal, found, value1)
	}

	resultNew := tx.newTr.Walk(math.MaxInt, nil)
	expectNew := []KVPair[int]{{[]byte("b"), value2}}
	if !reflect.DeepEqual(resultNew, expectNew) {
		t.Errorf("newTr.Walk got %v want %v", resultNew, expectNew)
	}

	resultOld := tx.oldTr.Walk(math.MaxInt, nil)
	expectOld := []KVPair[int]{
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
	tr := New[int]()
	tr.Upsert([]byte("a"), value1)
	tr.Upsert([]byte("b"), value2)
	tr.Upsert([]byte("c"), value1)

	tx := tr.Txn()
	if _, found := tx.Delete([]byte("b")); !found {
		t.Fatal("Delete(b) not found")
	}
	tx.Upsert([]byte("d"), value2)

	resultNew := tx.newTr.Walk(math.MaxInt, nil)
	expectNew := []KVPair[int]{
		{[]byte("a"), value1},
		{[]byte("c"), value1},
		{[]byte("d"), value2},
	}
	if !reflect.DeepEqual(resultNew, expectNew) {
		t.Errorf("newTr.Walk got %v want %v", resultNew, expectNew)
	}

	resultOld := tx.oldTr.Walk(math.MaxInt, nil)
	expectOld := []KVPair[int]{
		{[]byte("a"), value1},
		{[]byte("b"), value2},
		{[]byte("c"), value1},
	}
	if !reflect.DeepEqual(resultOld, expectOld) {
		t.Errorf("oldTr.Walk got %v want %v", resultOld, expectOld)
	}
}

func Test_CowOnInsert(t *testing.T) {
	onInsert := func(newVal int) int {
		return newVal * 10
	}
	tr := New(WithOnInsert(onInsert))

	tx := tr.Txn()
	tx.Upsert([]byte("a"), 5)

	val, found := tx.Get([]byte("a"))
	if !found || val != 50 {
		t.Errorf("Get(a) got %v found=%v want 50 true", val, found)
	}

	tr = tx.Commit()
	val, found = tr.Get([]byte("a"))
	if !found || val != 50 {
		t.Errorf("after commit Get(a) got %v found=%v want 50 true", val, found)
	}
}

func Test_CowOnUpdate(t *testing.T) {
	onUpdate := func(newVal, oldVal int) int {
		return newVal + oldVal
	}
	tr := New(WithOnUpdate(onUpdate))
	tr.Upsert([]byte("a"), 1)

	tx := tr.Txn()
	oldVal, isUpdate := tx.Upsert([]byte("a"), 2)
	if !isUpdate || oldVal != 1 {
		t.Fatalf("Upsert(a) got oldVal=%v isUpdate=%v want 1 true", oldVal, isUpdate)
	}

	val, found := tx.Get([]byte("a"))
	if !found || val != 3 {
		t.Errorf("Get(a) got %v found=%v want 3 true", val, found)
	}

	resultOld := tx.oldTr.Walk(math.MaxInt, nil)
	expectOld := []KVPair[int]{{[]byte("a"), 1}}
	if !reflect.DeepEqual(resultOld, expectOld) {
		t.Errorf("oldTr.Walk got %v want %v", resultOld, expectOld)
	}
}

func Test_CowCommit(t *testing.T) {
	tests := []struct {
		name    string
		oldTr   []KVPair[int]
		updates []updateItem
		deletes []deleteItem
		expectR []KVPair[int]
	}{
		{
			name:  "Empty cow",
			oldTr: []KVPair[int]{},
			updates: []updateItem{
				{[]byte("a"), value1, 0, false},
				{[]byte("b"), value1, 0, false},
			},
			deletes: []deleteItem{
				{[]byte("b"), value1, true},
				{[]byte("c"), 0, false},
			},
			expectR: []KVPair[int]{
				{[]byte("a"), value1},
			},
		},
		{
			name: "Simple cow",
			oldTr: []KVPair[int]{
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
				{[]byte("e"), 0, false},
			},
			expectR: []KVPair[int]{
				{[]byte("a"), value2},
				{[]byte("d"), value1},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tr := New[int]()
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
		oldTr   []KVPair[int]
		updates []updateItem
		deletes []deleteItem
		expectR []KVPair[int]
	}{
		{
			name:  "Empty cow",
			oldTr: []KVPair[int]{},
			updates: []updateItem{
				{[]byte("a"), value1, 0, false},
				{[]byte("b"), value1, 0, false},
			},
			deletes: []deleteItem{
				{[]byte("b"), value1, true},
				{[]byte("c"), 0, false},
			},
			expectR: nil,
		},
		{
			name: "Simple cow",
			oldTr: []KVPair[int]{
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
				{[]byte("e"), 0, false},
			},
			expectR: []KVPair[int]{
				{[]byte("a"), value1},
				{[]byte("b"), value1},
				{[]byte("c"), value1},
				{[]byte("d"), value1},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tr := New[int]()
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

// A transaction copies cow-marked nodes and must leave the mark on the
// originals. They still belong to the old trie, and without the mark a later
// write that reaches them would change them in place.
func Test_CowKeepsMarkOnOldNodes(t *testing.T) {
	tr := New[int]()
	for _, key := range []string{"a", "b", "c"} {
		tr.Upsert([]byte(key), value1)
	}

	tx := tr.Txn()
	tx.Upsert([]byte("a"), value2)
	tx.Delete([]byte("b"))
	tx.Upsert([]byte("d"), value1)

	root := tr.root.(*branchNode[int])
	if !root.cowMarked() {
		t.Error("old root lost its cow mark")
	}
	for _, twig := range root.twigs {
		if !twig.cowMarked() {
			t.Errorf("old leaf %q lost its cow mark", twig.(*leafNode[int]).key)
		}
	}
}

// Test_CowConcurrentReaders runs the supported concurrency model under the
// race detector: one writer commits transactions and publishes each new trie
// through an atomic pointer, while readers use whatever version they loaded.
func Test_CowConcurrentReaders(t *testing.T) {
	const (
		readers    = 4
		readerRuns = 50
		minRounds  = 100
	)
	alphabet := []byte{0x00, 0x01, 0x0f, 0x10, 0x61, 0x62, 0xf0, 0xff}
	randKey := func(r *rand.Rand) []byte {
		key := make([]byte, 1+r.Intn(4))
		for i := range key {
			key[i] = alphabet[r.Intn(len(alphabet))]
		}
		return key
	}

	r := rand.New(rand.NewSource(1))
	tr := New[int]()
	for i := range 1000 {
		tr.Upsert(randKey(r), i)
	}
	var cur atomic.Pointer[Trie[int]]
	cur.Store(tr)

	var wg sync.WaitGroup
	for g := range readers {
		wg.Add(1)
		go func() {
			defer wg.Done()
			r := rand.New(rand.NewSource(int64(100 + g)))
			for range readerRuns {
				snap := cur.Load()
				for range 50 {
					key := randKey(r)
					val, found := snap.Get(key)
					k, v, exact, leFound := snap.GetLessOrEqual(key)
					if exact != found || (found && (v != val || !bytes.Equal(k, key))) {
						t.Errorf("Get(%x) = %v, %v but GetLessOrEqual = %x, %v, %v", key, val, found, k, v, exact)
						return
					}
					if leFound != (k != nil) {
						t.Errorf("GetLessOrEqual(%x) = %x, found=%v", key, k, leFound)
						return
					}
					if !found && leFound && bytes.Compare(k, key) >= 0 {
						t.Errorf("GetLessOrEqual(%x) = %x, not a smaller key", key, k)
						return
					}
				}

				// A version never changes once it is published: a full
				// iteration is sorted and as long as Size says.
				var prev []byte
				n := 0
				it := snap.Iterator()
				for {
					k, _, ok := it.Next()
					if !ok {
						break
					}
					if prev != nil && bytes.Compare(prev, k) >= 0 {
						t.Errorf("iteration out of order: %x then %x", prev, k)
						return
					}
					prev = k
					n++
				}
				if n != snap.Size() {
					t.Errorf("iterated %d keys, Size() = %d", n, snap.Size())
					return
				}
			}
		}()
	}

	readersDone := make(chan struct{})
	go func() {
		wg.Wait()
		close(readersDone)
	}()

	// Keep writing until every reader has finished.
	for round := 0; ; round++ {
		if round >= minRounds {
			select {
			case <-readersDone:
				return
			default:
			}
		}
		tx := cur.Load().Txn()
		for i := range 30 {
			key := randKey(r)
			if r.Intn(3) == 0 {
				tx.Delete(key)
			} else {
				tx.Upsert(key, i)
			}
		}
		if r.Intn(5) == 0 {
			tx.Abort()
		} else {
			cur.Store(tx.Commit())
		}
	}
}
