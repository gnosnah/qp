package qp

import (
	"testing"
)

func Test_NibbleIndex(t *testing.T) {
	tests := []struct {
		name          string
		k1            []byte
		k2            []byte
		expectedIndex nibbleIndexT
		expectedMatch bool
	}{
		{
			name:          "Different single nibble keys",
			k1:            []byte{0x12},
			k2:            []byte{0x34},
			expectedIndex: 0,
			expectedMatch: false,
		},
		{
			name:          "Identical multi-nibble keys",
			k1:            []byte{0x12, 0x34},
			k2:            []byte{0x12, 0x34},
			expectedIndex: 4,
			expectedMatch: true,
		},
		{
			name:          "Partial match multi-nibble keys",
			k1:            []byte{0x12, 0x34},
			k2:            []byte{0x12, 0x56},
			expectedIndex: 2,
			expectedMatch: false,
		},
		{
			name:          "Different Length Keys with Partial Match",
			k1:            []byte{0x12, 0x34},
			k2:            []byte{0x12},
			expectedIndex: 2,
			expectedMatch: false,
		},
		{
			name:          "Different Length Keys with No Match",
			k1:            []byte{0x12, 0x34},
			k2:            []byte{0x56},
			expectedIndex: 0,
			expectedMatch: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			index, match := nibbleIndex(tt.k1, tt.k2)
			if tt.expectedIndex != index || tt.expectedMatch != match {
				t.Errorf("nibbleIndex(%v, %v) = (%d, %t), want (%d, %t)",
					tt.k1, tt.k2, index, match, tt.expectedIndex, tt.expectedMatch)
			}
		})
	}
}

func Test_NibbleBit(t *testing.T) {
	tests := []struct {
		name     string
		index    nibbleIndexT
		key      []byte
		expected bitmapT
	}{
		{
			name:     "Byte index out of bounds",
			index:    10,
			key:      []byte{0x12, 0x34},
			expected: bitmapT(1),
		},
		{
			name:     "Upper nibble of last byte",
			index:    2,
			key:      []byte{0x12, 0x34},
			expected: 1 << 0x3,
		},
		{
			name:     "Lower nibble of last byte",
			index:    3,
			key:      []byte{0x12, 0x34},
			expected: 1 << 0x4,
		},
		{
			name:     "Nibble Value Zero",
			index:    0,
			key:      []byte{0x00},
			expected: 1 << 0x0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := nibbleBit(tt.index, tt.key)
			if result != tt.expected {
				t.Errorf("nibbleBit(%d, %v) = %v, want %v", tt.index, tt.key, result, tt.expected)
			}
		})
	}
}
