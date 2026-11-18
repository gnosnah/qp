package qp

import (
	"bufio"
	"bytes"
	"fmt"
	"math"
	"math/rand"
	"os"
	"testing"
	"time"
)

func Test_OnInsert(t *testing.T) {
	onInsert := func(newVal int) (finalVal int) {
		v := newVal
		return v * 10
	}
	tr := New(WithOnInsert(onInsert))

	for i := 1; i < 10; i++ {
		key := fmt.Sprintf("%d", i)
		val := i
		tr.Upsert([]byte(string(key)), val)
	}

	for i := 1; i < 10; i++ {
		key := fmt.Sprintf("%d", i)
		val, found := tr.Get([]byte(string(key)))
		if !found {
			t.Fatalf("%s not exist", key)
		}

		if val != i*10 {
			t.Fatalf("key: %s, expected value: %d, got: %v", key, i*10, val)
		}
	}
}

func Test_OnUpdate(t *testing.T) {
	onUpdate := func(newVal, oldVal int) (finalVal int) {
		v := newVal
		return v + oldVal
	}
	tr := New(WithOnUpdate(onUpdate))

	for i := 1; i < 10; i++ {
		key := fmt.Sprintf("%d", i)
		val := i
		tr.Upsert([]byte(string(key)), val)
	}

	// update
	for i := 1; i < 10; i++ {
		key := fmt.Sprintf("%d", i)
		val := i
		tr.Upsert([]byte(string(key)), val)
	}

	for i := 1; i < 10; i++ {
		key := fmt.Sprintf("%d", i)
		val, found := tr.Get([]byte(string(key)))
		if !found {
			t.Fatalf("%s not exist", key)
		}

		if val != i*2 {
			t.Fatalf("key: %s, expected value: %d, got: %v", key, i*10, val)
		}
	}
}

func Test_GetEmpty(t *testing.T) {
	tr := New[int]()
	data := []string{"a", "b"}
	for _, d := range data {
		val, found := tr.Get([]byte(d))
		if found {
			t.Fatalf("%s should not exist", d)
		}
		if val != 0 {
			t.Fatalf("%s should be the zero value", d)
		}
	}
}

// A miss returns the zero value of the value type: nil for a pointer type,
// an empty struct for a struct type.
func Test_MissReturnsZeroValue(t *testing.T) {
	type record struct {
		ttl  uint32
		data string
	}
	miss := []byte("b")

	t.Run("pointer", func(t *testing.T) {
		tr := New[*record]()
		tr.Upsert([]byte("a"), &record{ttl: 1})
		tr.Upsert([]byte("c"), &record{ttl: 2})

		if v, found := tr.Get(miss); found || v != nil {
			t.Errorf("Get = %v, %v; want nil, false", v, found)
		}
		if v, found := tr.Delete(miss); found || v != nil {
			t.Errorf("Delete = %v, %v; want nil, false", v, found)
		}
		if v, isUpdate := tr.Upsert(miss, &record{ttl: 3}); isUpdate || v != nil {
			t.Errorf("Upsert = %v, %v; want nil, false", v, isUpdate)
		}
		if k, v, exact, found := tr.GetLessOrEqual([]byte("A")); found || k != nil || v != nil || exact {
			t.Errorf("GetLessOrEqual = %q, %v, %v, %v; want no key", k, v, exact, found)
		}

		// "b" is stored now, so the transaction needs another missing key.
		tx := tr.Txn()
		txMiss := []byte("d")
		if v, found := tx.Get(txMiss); found || v != nil {
			t.Errorf("Txn.Get = %v, %v; want nil, false", v, found)
		}
		if v, found := tx.Delete(txMiss); found || v != nil {
			t.Errorf("Txn.Delete = %v, %v; want nil, false", v, found)
		}
		if v, isUpdate := tx.Upsert(txMiss, &record{ttl: 4}); isUpdate || v != nil {
			t.Errorf("Txn.Upsert = %v, %v; want nil, false", v, isUpdate)
		}
	})

	t.Run("struct", func(t *testing.T) {
		var zero record
		tr := New[record]()
		tr.Upsert([]byte("a"), record{ttl: 1, data: "a"})
		tr.Upsert([]byte("c"), record{ttl: 2, data: "c"})

		if v, found := tr.Get(miss); found || v != zero {
			t.Errorf("Get = %v, %v; want zero value, false", v, found)
		}
		if v, found := tr.Delete(miss); found || v != zero {
			t.Errorf("Delete = %v, %v; want zero value, false", v, found)
		}
		if k, v, exact, found := tr.GetLessOrEqual([]byte("A")); found || k != nil || v != zero || exact {
			t.Errorf("GetLessOrEqual = %q, %v, %v, %v; want no key", k, v, exact, found)
		}

		it := tr.Iterator()
		it.Next()
		it.Next()
		if k, v, ok := it.Next(); ok || k != nil || v != zero {
			t.Errorf("Next past the end = %q, %v, %v; want no key", k, v, ok)
		}
	})
}

func Test_Get(t *testing.T) {
	data := []string{"a", "b", "c", "f", "cef", "e", "cefy"}
	tr := New[int]()
	for _, d := range data {
		tr.Upsert([]byte(d), value1)
	}

	for _, d := range data {
		val, found := tr.Get([]byte(d))
		if !found {
			t.Fatalf("%s not exist", d)
		}
		if val != value1 {
			t.Fatalf("%s val %d != value1 %d", d, val, value1)
		}
	}

	if tr.Size() != len(data) {
		t.Fatalf("size not match")
	}
}

func Test_Upsert(t *testing.T) {
	data := []string{"a", "b", "c", "f", "cef", "e", "cefy"}

	tr := New[int]()
	for _, d := range data {
		tr.Upsert([]byte(d), value1)
	}

	if tr.Size() != len(data) {
		t.Fatalf("size not match")
	}

	update := []string{"a", "b", "c"}
	for _, d := range update {
		oldVal, isUpdate := tr.Upsert([]byte(d), value2)
		if oldVal != value1 {
			t.Fatalf("oldVal not match")
		}
		if !isUpdate {
			t.Fatalf("isUpdate not match")
		}
	}
	if tr.Size() != len(data) {
		t.Fatalf("size not match")
	}

	insert := []string{"a1", "b1", "c1"}
	for _, d := range insert {
		_, isUpdate := tr.Upsert([]byte(d), value2)
		if isUpdate {
			t.Fatalf("isUpdate not match")
		}
	}
	if tr.Size() != len(data)+len(insert) {
		t.Fatalf("size not match")
	}
}

func Test_UpsertCopiesKey(t *testing.T) {
	tr := New[int]()
	buf := []byte("alpha")
	tr.Upsert(buf, value1)

	stored := tr.root.(*leafNode[int]).key
	if &stored[0] == &buf[0] {
		t.Fatal("root leaf aliases the caller buffer")
	}
	copy(buf, "brave")
	tr.Upsert(buf, value2)
	if aliasesCaller(tr.root, []byte("brave"), buf) {
		t.Fatal("inserted leaf aliases the caller buffer")
	}
	copy(buf, "xxxxx")

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

func aliasesCaller(n trieNode[int], key, buf []byte) bool {
	switch node := n.(type) {
	case *leafNode[int]:
		return bytes.Equal(node.key, key) && len(node.key) > 0 && &node.key[0] == &buf[0]
	case *branchNode[int]:
		for _, tw := range node.twigs {
			if tw != nil && aliasesCaller(tw, key, buf) {
				return true
			}
		}
	}
	return false
}

func Test_WordsSetGet(t *testing.T) {
	data := loadTestData(wordsSortedPath)

	tr := New[int]()
	for _, key := range data {
		oldVal, isUpdate := tr.Upsert(key, value1)
		if oldVal != 0 || isUpdate {
			t.Fatalf("set key: %s failed", string(key))
		}
	}

	for _, key := range data {
		val, found := tr.Get(key)
		if !found {
			t.Fatalf("key: %s not found", string(key))
		}
		if val != value1 {
			t.Fatalf("value not match, expect: %d, got: %v", value1, val)
		}
	}

	if tr.Size() != len(data) {
		t.Fatalf("size not match")
	}
}

func Test_RemoveTwigShrinksSlice(t *testing.T) {
	keys := make([][]byte, 4)
	for i := range keys {
		keys[i] = []byte{0x60 + byte(i)}
	}

	tr := New[int]()
	for _, key := range keys {
		tr.Upsert(key, value1)
	}
	assertTwigsTight(t, tr.root, 4)

	// Delete the first, middle, and last twig. Each removal must shrink the
	// slice and leave every surviving key readable.
	for _, idx := range []int{0, 1, 3} {
		if _, found := tr.Delete(keys[idx]); !found {
			t.Fatalf("Delete(%x) not found", keys[idx])
		}
		assertTwigsTight(t, tr.root, 3)
		if _, found := tr.Get(keys[idx]); found {
			t.Fatalf("Get(%x) still present", keys[idx])
		}
		for i, key := range keys {
			if i == idx {
				continue
			}
			if _, found := tr.Get(key); !found {
				t.Fatalf("Get(%x) missing after Delete(%x)", key, keys[idx])
			}
		}
		tr.Upsert(keys[idx], value1)
		assertTwigsTight(t, tr.root, 4)
	}

	const cycles = 1000
	for i := range cycles {
		if _, found := tr.Delete(keys[1]); !found {
			t.Fatalf("cycle %d: Delete not found", i)
		}
		tr.Upsert(keys[1], value2)
	}
	assertTwigsTight(t, tr.root, 4)
	if tr.Size() != len(keys) {
		t.Fatalf("Size = %d, want %d", tr.Size(), len(keys))
	}
	val, found := tr.Get(keys[1])
	if !found || val != value2 {
		t.Fatalf("Get(%x) = %v found=%v, want %d true", keys[1], val, found, value2)
	}
}

func assertTwigsTight(t *testing.T, n trieNode[int], wantLen int) {
	t.Helper()
	bn, ok := n.(*branchNode[int])
	if !ok {
		t.Fatalf("node is %T, want *branchNode[int]", n)
	}
	if len(bn.twigs) != wantLen || bn.twigOffsetMax() != wantLen {
		t.Fatalf("twigs len=%d live=%d, want %d", len(bn.twigs), bn.twigOffsetMax(), wantLen)
	}
	for i, tw := range bn.twigs {
		if tw == nil {
			t.Fatalf("nil twig at %d", i)
		}
	}
	if cap(bn.twigs) > len(bn.twigs) && bn.twigs[:cap(bn.twigs)][len(bn.twigs)] != nil {
		t.Fatal("backing array retains a twig past len")
	}
}

func Test_DeleteEmpty(t *testing.T) {
	tr := New[int]()
	data := []string{"a", "b"}
	for _, d := range data {
		oldVal, found := tr.Delete([]byte(d))
		if found {
			t.Fatalf("%s should not exist", d)
		}
		if oldVal != 0 {
			t.Fatalf("%s should be the zero value", d)
		}
	}
}

func Test_RandomString(t *testing.T) {
	start := time.Now()
	kvs := make(map[string]string)
	maxSize := 10000
	for i := 0; i < maxSize; i++ {
		kvs[randString()] = randString()
	}
	elapsed := time.Since(start).Milliseconds()
	t.Logf("generated kvs size:%d, took:%d(ms)", len(kvs), elapsed)

	start = time.Now()
	tr := New[[]byte]()
	for k, v := range kvs {
		tr.Upsert([]byte(k), []byte(v))
	}
	if tr.Size() != len(kvs) {
		t.Errorf("tr.Size() = %d, want %d", tr.Size(), len(kvs))
	}
	elapsed = time.Since(start).Milliseconds()
	t.Logf("Set took:%d(ms)", elapsed)

	start = time.Now()
	for k, v := range kvs {
		val, ok := tr.Get([]byte(k))
		if !ok {
			t.Errorf("tr.Get(%s) should exist", k)
		}
		if !bytes.Equal(val, []byte(v)) {
			t.Errorf("got: %v, expected: %v", val, v)
		}
	}
	elapsed = time.Since(start).Milliseconds()
	t.Logf("Get took:%d(ms)", elapsed)

	start = time.Now()
	for k, v := range kvs {
		val, ok := tr.Delete([]byte(k))
		if !ok {
			t.Errorf("tr.Delete(%s) should exist", k)
		}
		if !bytes.Equal(val, []byte(v)) {
			t.Errorf("got: %v, expected: %v", val, v)
		}
	}
	elapsed = time.Since(start).Milliseconds()
	t.Logf("Delete took:%d(ms)", elapsed)

	if tr.Size() != 0 {
		t.Errorf("tr.Size() = %d, want %d after Delete", tr.Size(), 0)
	}
}

// A nil node can only come from a corrupted trie. Every operation that walks
// the trie must panic on it; without a default case the walk loops forever.
func Test_NilNodePanics(t *testing.T) {
	tests := []struct {
		name string
		op   func(tr *Trie[int])
	}{
		{"Get", func(tr *Trie[int]) { tr.Get([]byte("b")) }},
		{"Upsert", func(tr *Trie[int]) { tr.Upsert([]byte("b"), value2) }},
		{"Delete", func(tr *Trie[int]) { tr.Delete([]byte("b")) }},
		{"GetLessOrEqual on the key", func(tr *Trie[int]) { tr.GetLessOrEqual([]byte("b")) }},
		// "c" is not stored; the walk for its predecessor ends in the nil twig.
		{"GetLessOrEqual on the next key", func(tr *Trie[int]) { tr.GetLessOrEqual([]byte("c")) }},
		{"Walk", func(tr *Trie[int]) { tr.Walk(math.MaxInt, nil) }},
		{"Txn.Upsert", func(tr *Trie[int]) { tr.Txn().Upsert([]byte("b"), value2) }},
		{"Txn.Delete", func(tr *Trie[int]) { tr.Txn().Delete([]byte("b")) }},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tr := New[int]()
			for _, key := range []string{"a", "b", "d"} {
				tr.Upsert([]byte(key), value1)
			}
			// The root is one branch with three leaves. Drop the leaf of "b".
			tr.root.(*branchNode[int]).twigs[1] = nil

			panicked := make(chan bool, 1)
			go func() {
				defer func() { panicked <- recover() != nil }()
				tt.op(tr)
			}()
			select {
			case p := <-panicked:
				if !p {
					t.Fatal("returned normally on a nil node")
				}
			case <-time.After(5 * time.Second):
				t.Fatal("still running after 5s on a nil node")
			}
		})
	}
}

func Test_GetLessOrEqualReturnedKeyDoesNotAliasQuery(t *testing.T) {
	for _, query := range []string{"a", "b"} {
		t.Run(query, func(t *testing.T) {
			tr := New[int]()
			tr.Upsert([]byte("a"), value1)
			tr.Upsert([]byte("c"), value2)
			buf := []byte(query)
			key, val, exact, found := tr.GetLessOrEqual(buf)
			if !found || string(key) != "a" || val != value1 || exact != (query == "a") {
				t.Fatalf("GetLessOrEqual(%q) = %q, %v, %v, %v", query, key, val, exact, found)
			}
			// Reuse the caller-owned query buffer without touching the result.
			buf[0] = 'z'
			if string(key) != "a" {
				t.Fatalf("reusing the query buffer changed the returned key to %q", key)
			}
			if val, found := tr.Get(key); !found || val != value1 {
				t.Fatalf("Get(returned key) = %v, %v; want %v, true", val, found, value1)
			}
		})
	}
}

func Test_GetLessOrEqual(t *testing.T) {
	type expects struct {
		searchK            string
		expectedK          string
		expectedExactMatch bool
	}

	tests := []struct {
		name  string
		data  []string
		items []expects
	}{
		{
			name: "empty trie",
			data: []string{},
			items: []expects{
				{"a", "", false},
				{"b", "", false},
			},
		},
		{
			name: "simple trie",
			data: []string{"a", "b", "c", "f", "cef", "e", "cefy"},
			items: []expects{
				{"A", "", false},
				{"d", "cefy", false},
				{"f", "f", true},
				{"mnxy", "f", false},
				{"cf", "cefy", false},
				{"cek", "cefy", false},
				{"cef", "cef", true},
				{"ceka", "cefy", false},
			},
		},
		{
			name: "simple trie with no byte",
			data: []string{"c\000a", "c\000a\000b", "c\000b", "c\000abc", "d\000a", "e\000abc"},
			items: []expects{
				{"c\000a\000b\001", "c\000a\000b", false},
				{"c\000a\000abc", "c\000a", false},
				{"c\000a", "c\000a", true},
				{"c\000c", "c\000b", false},
				{"e\000a", "d\000a", false},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tr := New[int]()
			for _, d := range tt.data {
				tr.Upsert([]byte(d), value1)
			}
			for _, item := range tt.items {
				leKey := ""
				k, _, exactMatch, found := tr.GetLessOrEqual([]byte(item.searchK))
				if exactMatch != item.expectedExactMatch {
					t.Errorf("exactMatch = %v, want %v", exactMatch, item.expectedExactMatch)
				}
				// No test case stores an empty key, so "" stands for no key.
				if wantFound := item.expectedK != ""; found != wantFound {
					t.Errorf("search: %s, found = %v, want %v", item.searchK, found, wantFound)
				}
				if found {
					leKey = string(k)
				}
				if item.expectedK != leKey {
					t.Fatalf("search: %s, expect: %s, got: %s", item.searchK, item.expectedK, leKey)
				}
			}
		})
	}
}

func Benchmark_Words_Upsert(b *testing.B) {
	words := loadTestData(wordsPath)

	for b.Loop() {
		tr := New[[]byte]()
		for _, w := range words {
			tr.Upsert(w, w)
		}
	}
}

func Benchmark_Words_Get(b *testing.B) {
	words := loadTestData(wordsPath)
	tr := New[[]byte]()
	for _, w := range words {
		tr.Upsert(w, w)
	}

	for b.Loop() {
		for _, w := range words {
			val, ok := tr.Get(w)
			if !ok {
				b.Fatalf("failed to get a value from the tree. key: %v", w)
			}
			if !bytes.Equal(val, w) {
				b.Fatalf("returned value does not match an expected one. got: %v, expected: %v", val, w)
			}
		}
	}
}

func Benchmark_UUID_Upsert(b *testing.B) {
	uuids := loadTestData(uuidPath)

	for b.Loop() {
		tr := New[[]byte]()
		for _, id := range uuids {
			tr.Upsert(id, id)
		}
	}
}

func Benchmark_UUID_Get(b *testing.B) {
	uuids := loadTestData(uuidPath)
	tr := New[[]byte]()
	for _, id := range uuids {
		tr.Upsert(id, id)
	}

	for b.Loop() {
		for _, id := range uuids {
			val, ok := tr.Get(id)
			if !ok {
				b.Fatalf("failed to get a value from the tree. key: %v", id)
			}
			if !bytes.Equal(val, id) {
				b.Fatalf("returned value does not match an expected one. got: %v, expected: %v", val, id)
			}
		}
	}
}

const (
	value1 = 1
	value2 = 2

	wordsPath       = "testdata/words.txt"
	wordsSortedPath = "testdata/words_sorted.txt"
	uuidPath        = "testdata/uuid.txt"
)

func loadTestData(path string) [][]byte {
	f, err := os.Open(path)
	if err != nil {
		panic("open err: " + err.Error())
	}
	defer func() {
		_ = f.Close()
	}()

	var data [][]byte
	rd := bufio.NewReader(f)
	for {
		line, err := rd.ReadBytes('\n')
		if len(line) > 0 {
			if line[len(line)-1] == '\n' {
				data = append(data, line[:len(line)-1])
			} else {
				data = append(data, line)
			}
		}
		if err != nil {
			break
		}
	}
	return data
}

func randString() string {
	const letterBytes = "abcdefghijklmnopqrstuvwxyz0123456789-.~!@#$%^&*()<>?;'"
	const maxSize = 256
	rd := rand.New(rand.NewSource(time.Now().Unix()))
	n := rd.Intn(maxSize)
	if n == 0 {
		n = 1
	}
	b := make([]byte, n)
	for i := range b {
		b[i] = letterBytes[rand.Intn(len(letterBytes))]
	}
	return string(b)
}
