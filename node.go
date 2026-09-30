package qp

import "math/bits"

type trieNode interface {
	dup() trieNode
	cowMarked() bool
	markCow()
	clearCow()
}

type cowFlag struct {
	cow bool
}

func (c *cowFlag) cowMarked() bool { return c.cow }
func (c *cowFlag) markCow()        { c.cow = true }
func (c *cowFlag) clearCow()       { c.cow = false }

type leafNode struct {
	cowFlag
	key   []byte
	value any
}

func (ln *leafNode) dup() trieNode {
	return &leafNode{key: ln.key, value: ln.value}
}

type branchNode struct {
	cowFlag
	twigs  []trieNode   // up to 17 twigs, 0th is NO_BYTE
	bitmap bitmapT      // store which slot is not-NULL
	index  nibbleIndexT // nibble index, start from 0
}

func (bn *branchNode) dup() trieNode {
	// the copy shares the twigs with the original, so they become
	// copy-on-write as well
	bn.markTwigs()

	newBn := branchNode{
		twigs:  make([]trieNode, len(bn.twigs)),
		bitmap: bn.bitmap,
		index:  bn.index,
	}
	copy(newBn.twigs, bn.twigs)
	return &newBn
}

func (bn *branchNode) markTwigs() {
	for i := 0; i < bn.twigOffsetMax(); i++ {
		bn.twigs[i].markCow()
	}
}

func (bn *branchNode) hasTwig(b bitmapT) bool {
	return bn.bitmap&b > 0
}

func (bn *branchNode) twigOffset(b bitmapT) int {
	w := bn.bitmap & (b - 1)
	return bits.OnesCount32(w)
}

func (bn *branchNode) twig(i int) *trieNode {
	return &bn.twigs[i]
}

func (bn *branchNode) twigOffsetMax() int {
	return bits.OnesCount32(bn.bitmap)
}

func (bn *branchNode) twigBit(key []byte) bitmapT {
	return nibbleBit(bn.index, key)
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
	last := len(bn.twigs) - 1
	copy(bn.twigs[twigOffset:], bn.twigs[twigOffset+1:])
	bn.twigs[last] = nil
	bn.twigs = bn.twigs[:last]
	bn.bitmap &= ^b
}

// uncow replaces *ptr with a private copy when the node is still shared with
// another trie, so that the caller may modify it in place. It returns the node
// stored at ptr afterwards.
func uncow(ptr *trieNode) trieNode {
	n := *ptr
	if !n.cowMarked() {
		return n
	}
	n.clearCow()
	n = n.dup()
	*ptr = n
	return n
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
