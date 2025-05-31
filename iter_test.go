package qp

import (
	"bytes"
	"testing"
)

func Test_SimpleIter(t *testing.T) {
	tests := []struct {
		name     string
		data     []string
		expected []string
	}{
		{
			name:     "empty iter",
			data:     []string{},
			expected: []string{},
		},
		{
			name:     "one item iter",
			data:     []string{"a"},
			expected: []string{"a"},
		},
		{
			name:     "two items iter",
			data:     []string{"a", "aa"},
			expected: []string{"a", "aa"},
		},
		{
			name:     "simple iter",
			data:     []string{"b", "a", "c", "f", "cef", "e", "cefy"},
			expected: []string{"a", "b", "c", "cef", "cefy", "e", "f"},
		},
		{
			name:     "no byte iter case1",
			data:     []string{"c\001\000a", "c\000a", "d\000ac", "d\000a", "d\000aa", "d\000a\001"},
			expected: []string{"c\000a", "c\001\000a", "d\000a", "d\000a\001", "d\000aa", "d\000ac"},
		},
		{
			name:     "no byte iter case2",
			data:     []string{"d\000a\001", "d\000a", "d\000a\002", "d\000a\003"},
			expected: []string{"d\000a", "d\000a\001", "d\000a\002", "d\000a\003"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tr := New()
			for idx := 0; idx < len(tt.data); idx++ {
				tr.Upsert([]byte(tt.data[idx]), value1)
			}

			var result []string
			it := tr.Iterator()
			for {
				k, v, ok := it.Next()
				if !ok {
					break
				}
				key := string(k)
				if v.(int) != value1 {
					t.Fatalf("key: %s, val %d != %d", key, v.(int), value1)
				}

				result = append(result, key)
			}

			if len(result) != len(tt.expected) {
				t.Fatalf("result size not match")
			}

			for i, r := range result {
				if r != tt.expected[i] {
					t.Fatalf("result not match")
				}
			}
		})
	}
}

func Test_WordsIter(t *testing.T) {
	words := loadTestData(wordsPath)
	tr := New()
	for _, word := range words {
		tr.Upsert(word, value1)
	}

	if tr.Size() != len(words) {
		t.Fatalf("upsert size not match")
	}

	sortedWords := loadTestData(wordsSortedPath)

	idx := 0
	it := tr.Iterator()
	for {
		k, _, ok := it.Next()
		if !ok {
			break
		}
		expect := sortedWords[idx]
		equal := bytes.Equal(expect, k)
		if !equal {
			t.Errorf("expect: %s, got: %s", string(expect), string(k))
		}
		idx++
	}

	if idx != len(words) {
		t.Fatalf("iter counter not match")
	}
}
