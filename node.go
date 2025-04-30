package qp

import "math/bits"

type trieNode interface {
	isBranch() bool
	// cow
	dup() trieNode
	cowMarked() bool
	markCow()
	clearCow()
}

type leafNode struct {
	key   []byte
	value any
	cow   bool
}

func (*leafNode) isBranch() bool {
	return false
}

func (ln *leafNode) dup() trieNode {
	return &leafNode{key: ln.key, value: ln.value, cow: ln.cow}
}

func (ln *leafNode) cowMarked() bool {
	return ln.cow
}

func (ln *leafNode) markCow() {
	ln.cow = true
}

func (ln *leafNode) clearCow() {
	ln.cow = false
}

type branchNode struct {
	twigs  []trieNode
	index  nibbleIndexT // nibble index, start from 0
	bitmap bitmapT      // store which slot is not-NULL
	// cow
	cow bool
}

func (*branchNode) isBranch() bool {
	return true
}

func (bn *branchNode) dup() trieNode {
	var newBn branchNode
	newBn.twigs = make([]trieNode, len(bn.twigs))
	copy(newBn.twigs, bn.twigs)
	newBn.index = bn.index
	newBn.bitmap = bn.bitmap
	newBn.cow = bn.cow
	return &newBn
}

func (bn *branchNode) markTwigs() {
	for _, twig := range bn.twigs {
		twig.markCow()
	}
}

func (bn *branchNode) cowMarked() bool {
	return bn.cow
}

func (bn *branchNode) markCow() {
	bn.cow = true
}

func (bn *branchNode) clearCow() {
	bn.cow = false
}

func (bn *branchNode) hasTwig(b bitmapT) bool {
	return bn.bitmap&b > 0
}

func (bn *branchNode) twigOffset(b bitmapT) int {
	w := bn.bitmap & (b - 1)
	return bits.OnesCount16(uint16(w))
}

func (bn *branchNode) twig(i int) *trieNode {
	return &bn.twigs[i]
}

func (bn *branchNode) twigOffsetMax() int {
	return bits.OnesCount16(uint16(bn.bitmap))
}

func (bn *branchNode) twigBit(key []byte) bitmapT {
	return nibbleBit(bn.index, key)
}

func (bn *branchNode) twigIdx(child trieNode) int {
	for i := range bn.twigs {
		if child == bn.twigs[i] {
			return i
		}
	}
	return -1
}

func (bn *branchNode) twigTail(twigIdx int) bool {
	return twigIdx+1 >= bn.twigOffsetMax()
}

func (bn *branchNode) growTwigs(index nibbleIndexT, newKey []byte, newLeaf *leafNode) {
	b := nibbleBit(index, newKey)
	twigOffset := bn.twigOffset(b)
	bn.twigs = append(bn.twigs, nil)
	copy(bn.twigs[twigOffset+1:], bn.twigs[twigOffset:])
	bn.twigs[twigOffset] = newLeaf
	bn.bitmap |= b
}

func (bn *branchNode) removeTwig(b bitmapT) {
	twigOffset := bn.twigOffset(b)
	copy(bn.twigs[twigOffset:], bn.twigs[twigOffset+1:])
	bn.twigs[len(bn.twigs)-1] = nil
	bn.bitmap &= ^b
}

func newBranchNode(n trieNode, index nibbleIndexT, oldKey, newKey []byte, newLeaf *leafNode) *branchNode {
	var bn branchNode
	b1 := nibbleBit(index, newKey)
	b2 := nibbleBit(index, oldKey)
	bn.twigs = make([]trieNode, 2)
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
