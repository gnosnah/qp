package qp

import (
	"bytes"
	"reflect"
	"testing"
)

func Test_Walk(t *testing.T) {
	tests := []struct {
		name    string
		data    []KVPair
		f       WalkFn
		max     int
		expectR []KVPair
	}{
		{
			name:    "empty trie walk",
			data:    []KVPair{},
			f:       defaultWalkFn,
			max:     10,
			expectR: nil,
		},
		{
			name: "prefix_a max 0 trie walk",
			data: []KVPair{
				{[]byte("mn"), value1},
				{[]byte("a"), value1},
				{[]byte("ab"), value1},
				{[]byte("xyz"), value1},
				{[]byte("d"), value1},
			},
			f:       func(key []byte, val any) bool { return bytes.HasPrefix(key, []byte("a")) },
			max:     0,
			expectR: nil,
		},
		{
			name: "prefix_a max 1 trie walk",
			data: []KVPair{
				{[]byte("mn"), value1},
				{[]byte("a"), value1},
				{[]byte("ab"), value1},
				{[]byte("abc"), value1},
				{[]byte("xyz"), value1},
				{[]byte("d"), value1},
			},
			f:   func(key []byte, val any) bool { return bytes.HasPrefix(key, []byte("a")) },
			max: 1,
			expectR: []KVPair{
				{[]byte("a"), value1},
			},
		},
		{
			name: "prefix_a max 2 trie walk",
			data: []KVPair{
				{[]byte("mn"), value1},
				{[]byte("a"), value1},
				{[]byte("ab"), value1},
				{[]byte("abc"), value1},
				{[]byte("xyz"), value1},
				{[]byte("d"), value1},
			},
			f:   func(key []byte, val any) bool { return bytes.HasPrefix(key, []byte("a")) },
			max: 2,
			expectR: []KVPair{
				{[]byte("a"), value1},
				{[]byte("ab"), value1},
			},
		},
		{
			name: "prefix_a max 10 trie walk",
			data: []KVPair{
				{[]byte("mn"), value1},
				{[]byte("a"), value1},
				{[]byte("ab"), value1},
				{[]byte("abc"), value1},
				{[]byte("xyz"), value1},
				{[]byte("d"), value1},
			},
			f:   func(key []byte, val any) bool { return bytes.HasPrefix(key, []byte("a")) },
			max: 10,
			expectR: []KVPair{
				{[]byte("a"), value1},
				{[]byte("ab"), value1},
				{[]byte("abc"), value1},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tr := New()
			for _, d := range tt.data {
				tr.Upsert([]byte(d.Key), d.Value)
			}

			result := tr.Walk(tt.max, tt.f)
			if !reflect.DeepEqual(result, tt.expectR) {
				t.Errorf("Walk got %v want %v", result, tt.expectR)
			}
		})
	}
}
