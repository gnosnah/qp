package qp

import (
	"bytes"
	"math/bits"
)

type trieNode[V any] interface {
	// cow
	dup() trieNode[V]
	cowMarked() bool
	markCow()
}

type leafNode[V any] struct {
	key   []byte
	value V
	cow   bool
}

func newLeafNode[V any](key []byte, value V) *leafNode[V] {
	return &leafNode[V]{key: bytes.Clone(key), value: value}
}

func (ln *leafNode[V]) dup() trieNode[V] {
	return &leafNode[V]{key: ln.key, value: ln.value}
}

func (ln *leafNode[V]) cowMarked() bool {
	return ln.cow
}

func (ln *leafNode[V]) markCow() {
	ln.cow = true
}

type branchNode[V any] struct {
	twigs  []trieNode[V] // up to 17 twigs, 0th is NO_BYTE
	bitmap bitmapT       // store which slot is not-NULL
	index  nibbleIndexT  // nibble index, start from 0
	// cow
	cow bool
}

func (bn *branchNode[V]) dup() trieNode[V] {
	var newBn branchNode[V]
	newBn.twigs = make([]trieNode[V], len(bn.twigs))
	copy(newBn.twigs, bn.twigs)
	newBn.index = bn.index
	newBn.bitmap = bn.bitmap
	return &newBn
}

func (bn *branchNode[V]) markTwigs() {
	for i := 0; i < bn.twigOffsetMax(); i++ {
		bn.twigs[i].markCow()
	}
}

func (bn *branchNode[V]) cowMarked() bool {
	return bn.cow
}

func (bn *branchNode[V]) markCow() {
	bn.cow = true
}

func (bn *branchNode[V]) hasTwig(b bitmapT) bool {
	return bn.bitmap&b > 0
}

func (bn *branchNode[V]) twigOffset(b bitmapT) int {
	w := bn.bitmap & (b - 1)
	return bits.OnesCount16(uint16(w))
}

func (bn *branchNode[V]) twig(i int) *trieNode[V] {
	return &bn.twigs[i]
}

func (bn *branchNode[V]) twigOffsetMax() int {
	return bits.OnesCount32(bn.bitmap)
}

func (bn *branchNode[V]) twigBit(key []byte) bitmapT {
	return nibbleBit(bn.index, key)
}

func (bn *branchNode[V]) twigIdx(child trieNode[V]) int {
	for i := range bn.twigs {
		if child == bn.twigs[i] {
			return i
		}
	}
	return -1
}

func (bn *branchNode[V]) twigTail(twigIdx int) bool {
	return twigIdx+1 >= bn.twigOffsetMax()
}

func (bn *branchNode[V]) growTwigs(index nibbleIndexT, newKey []byte, newLeaf *leafNode[V]) {
	b := nibbleBit(index, newKey)
	twigOffset := bn.twigOffset(b)
	bn.twigs = append(bn.twigs, nil)
	copy(bn.twigs[twigOffset+1:], bn.twigs[twigOffset:])
	bn.twigs[twigOffset] = newLeaf
	bn.bitmap |= b
}

func (bn *branchNode[V]) removeTwig(b bitmapT) {
	twigOffset := bn.twigOffset(b)
	copy(bn.twigs[twigOffset:], bn.twigs[twigOffset+1:])
	// Nil the vacated slot before shrinking so the backing array does not
	// retain it, then drop the slot so delete/insert cycles stay bounded.
	bn.twigs[len(bn.twigs)-1] = nil
	bn.twigs = bn.twigs[:len(bn.twigs)-1]
	bn.bitmap &= ^b
}

func newBranchNode[V any](n trieNode[V], index nibbleIndexT, oldKey, newKey []byte, newLeaf *leafNode[V]) *branchNode[V] {
	var bn branchNode[V]
	b1 := nibbleBit(index, newKey)
	b2 := nibbleBit(index, oldKey)
	bn.twigs = make([]trieNode[V], 2)
	bn.index = index
	bn.bitmap = b1 | b2

	if b1 < b2 {
		bn.twigs[0] = newLeaf
		bn.twigs[1] = n
	} else {
		bn.twigs[0] = n
		bn.twigs[1] = newLeaf
	}
	return &bn
}
