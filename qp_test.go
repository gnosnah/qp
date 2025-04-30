package qp

import (
	"bufio"
	"bytes"
	"fmt"
	"math/rand"
	"os"
	"testing"
	"time"
)

func Test_OnInsert(t *testing.T) {
	onInsert := func(newVal any) (finalVal any) {
		v := newVal.(int)
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

		if val.(int) != i*10 {
			t.Fatalf("key: %s, expected value: %d, got: %v", key, i*10, val)
		}
	}
}

func Test_OnUpdate(t *testing.T) {
	onUpdate := func(newVal, oldVal any) (finalVal any) {
		v := newVal.(int)
		return v + oldVal.(int)
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

		if val.(int) != i*2 {
			t.Fatalf("key: %s, expected value: %d, got: %v", key, i*10, val)
		}
	}
}

func Test_GetEmpty(t *testing.T) {
	tr := New()
	data := []string{"a", "b"}
	for _, d := range data {
		val, found := tr.Get([]byte(d))
		if found {
			t.Fatalf("%s should not exist", d)
		}
		if val != nil {
			t.Fatalf("%s should be nil", d)
		}
	}
}

func Test_Get(t *testing.T) {
	data := []string{"a", "b", "c", "f", "cef", "e", "cefy"}
	tr := New()
	for _, d := range data {
		tr.Upsert([]byte(d), value1)
	}

	for _, d := range data {
		v, found := tr.Get([]byte(d))
		if !found {
			t.Fatalf("%s not exist", d)
		}
		val := v.(int)
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

	tr := New()
	for _, d := range data {
		tr.Upsert([]byte(d), value1)
	}

	if tr.Size() != len(data) {
		t.Fatalf("size not match")
	}

	update := []string{"a", "b", "c"}
	for _, d := range update {
		oldVal, isUpdate := tr.Upsert([]byte(d), value2)
		if oldVal.(int) != value1 {
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

func Test_WordsSetGet(t *testing.T) {
	data := loadTestData(wordsSortedPath)

	tr := New()
	for _, key := range data {
		oldVal, isUpdate := tr.Upsert(key, value1)
		if oldVal != nil || isUpdate {
			t.Fatalf("set key: %s failed", string(key))
		}
	}

	for _, key := range data {
		val, found := tr.Get(key)
		if !found {
			t.Fatalf("key: %s not found", string(key))
		}
		if val.(int) != value1 {
			t.Fatalf("value not match, expect: %d, got: %v", value1, val)
		}
	}

	if tr.Size() != len(data) {
		t.Fatalf("size not match")
	}
}

func Test_DeleteEmpty(t *testing.T) {
	tr := New()
	data := []string{"a", "b"}
	for _, d := range data {
		oldVal, found := tr.Delete([]byte(d))
		if found {
			t.Fatalf("%s should not exist", d)
		}
		if oldVal != nil {
			t.Fatalf("%s should be nil", d)
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
	tr := New()
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
		if !bytes.Equal(val.([]byte), []byte(v)) {
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
		if !bytes.Equal(val.([]byte), []byte(v)) {
			t.Errorf("got: %v, expected: %v", val, v)
		}
	}
	elapsed = time.Since(start).Milliseconds()
	t.Logf("Delete took:%d(ms)", elapsed)

	if tr.Size() != 0 {
		t.Errorf("tr.Size() = %d, want %d after Delete", tr.Size(), 0)
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
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tr := New()
			for _, d := range tt.data {
				tr.Upsert([]byte(d), value1)
			}
			for _, item := range tt.items {
				leKey := ""
				k, _, exactMatch := tr.GetLessOrEqual([]byte(item.searchK))
				if exactMatch != item.expectedExactMatch {
					t.Errorf("exactMatch = %v, want %v", exactMatch, item.expectedExactMatch)
				}
				if k != nil {
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
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		tr := New()
		for _, w := range words {
			tr.Upsert(w, w)
		}
	}
}

func Benchmark_Words_Get(b *testing.B) {
	words := loadTestData(wordsPath)
	tr := New()
	for _, w := range words {
		tr.Upsert(w, w)
	}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		for _, w := range words {
			val, ok := tr.Get(w)
			if !ok {
				b.Fatalf("failed to get a value from the tree. key: %v", w)
			}
			if !bytes.Equal(val.([]byte), w) {
				b.Fatalf("returned value does not match an expected one. got: %v, expected: %v", val, w)
			}
		}
	}
}

func Benchmark_UUID_Upsert(b *testing.B) {
	uuids := loadTestData(uuidPath)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		tr := New()
		for _, id := range uuids {
			tr.Upsert(id, id)
		}
	}
}

func Benchmark_UUID_Get(b *testing.B) {
	uuids := loadTestData(uuidPath)
	tr := New()
	for _, id := range uuids {
		tr.Upsert(id, id)
	}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		for _, id := range uuids {
			val, ok := tr.Get(id)
			if !ok {
				b.Fatalf("failed to get a value from the tree. key: %v", id)
			}
			if !bytes.Equal(val.([]byte), id) {
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
	defer f.Close()

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
