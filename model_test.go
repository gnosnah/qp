package qp

import (
	"bytes"
	"errors"
	"fmt"
	"math/rand"
	"sort"
	"testing"
)

// The tests in this file drive the trie with random operations and compare it
// with a plain map. Seeds are fixed and every failure message names its seed,
// so a failure can be reproduced.

type trieModel map[string]int

func (m trieModel) clone() trieModel {
	c := make(trieModel, len(m))
	for k, v := range m {
		c[k] = v
	}
	return c
}

func (m trieModel) sortedKeys() []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

// trieWriter is the part of the API that Trie and Txn share.
type trieWriter interface {
	Get(key []byte) (val int, found bool)
	Upsert(key []byte, value int) (oldVal int, isUpdate bool)
	Delete(key []byte) (oldVal int, found bool)
}

type keyGen struct {
	name  string
	gen   func(r *rand.Rand) []byte
	seeds int
	ops   int // random operations used to build the trie
	// Shape that the direct model test must reach with this generator.
	minTwigs int
	minDepth int
}

var modelKeyGens = []keyGen{
	{
		// A few byte values on both sides of the nibble boundaries, 1 to 5
		// bytes long. Many keys are prefixes of other keys.
		name: "prefixes",
		gen: func(r *rand.Rand) []byte {
			alphabet := []byte{0x00, 0x01, 0x0f, 0x10, 0x61, 0x62, 0xf0, 0xff}
			key := make([]byte, 1+r.Intn(5))
			for i := range key {
				key[i] = alphabet[r.Intn(len(alphabet))]
			}
			return key
		},
		seeds: 20,
		ops:   300,
	},
	{
		// 'a' or 'b' followed by up to two bytes of any value. The branch
		// after the first byte gets all 17 twigs, the NO_BYTE one included.
		name: "wide",
		gen: func(r *rand.Rand) []byte {
			key := make([]byte, 1+r.Intn(3))
			key[0] = byte('a' + r.Intn(2))
			for i := 1; i < len(key); i++ {
				key[i] = byte(r.Intn(256))
			}
			return key
		},
		seeds:    6,
		ops:      1500,
		minTwigs: 17,
	},
	{
		// A run of one byte value, up to 200 bytes long, whose last byte may
		// differ in the upper or the lower nibble. The trie branches at
		// nearly every nibble and gets deeper than the iterator's first stack.
		name: "deep",
		gen: func(r *rand.Rand) []byte {
			key := bytes.Repeat([]byte{0xab}, 1+r.Intn(200))
			switch r.Intn(3) {
			case 0:
				key[len(key)-1] = 0x0b
			case 1:
				key[len(key)-1] = 0xa0
			}
			return key
		},
		seeds:    2,
		ops:      3000,
		minDepth: initIterStackSize + 1,
	},
}

func randomKeys(r *rand.Rand, gen func(r *rand.Rand) []byte, n int) [][]byte {
	keys := make([][]byte, n)
	for i := range keys {
		keys[i] = gen(r)
	}
	return keys
}

// applyRandomOps runs n random operations on w and on the model and compares
// what each operation returns.
func applyRandomOps(t *testing.T, tag string, r *rand.Rand, w trieWriter, m trieModel, gen func(r *rand.Rand) []byte, n int) {
	t.Helper()
	for range n {
		key := gen(r)
		want, wantFound := m[string(key)]
		switch r.Intn(4) {
		case 0:
			old, found := w.Delete(key)
			if found != wantFound || (found && old != want) || (!found && old != 0) {
				t.Fatalf("%s: Delete(%x) = %v, %v; want %v, %v", tag, key, old, found, want, wantFound)
			}
			delete(m, string(key))
		case 1:
			val, found := w.Get(key)
			if found != wantFound || (found && val != want) || (!found && val != 0) {
				t.Fatalf("%s: Get(%x) = %v, %v; want %v, %v", tag, key, val, found, want, wantFound)
			}
		default:
			val := r.Int()
			old, isUpdate := w.Upsert(key, val)
			if isUpdate != wantFound || (isUpdate && old != want) || (!isUpdate && old != 0) {
				t.Fatalf("%s: Upsert(%x) = %v, %v; want %v, %v", tag, key, old, isUpdate, want, wantFound)
			}
			m[string(key)] = val
		}
	}
}

// checkStructure verifies what every branch of a qp-trie must satisfy.
func checkStructure(n trieNode[int], parentIndex int) error {
	if n == nil {
		return errors.New("nil node")
	}
	bn, ok := n.(*branchNode[int])
	if !ok {
		return nil
	}
	if len(bn.twigs) != bn.twigOffsetMax() {
		return fmt.Errorf("branch at index %d has %d twigs for %d bitmap bits", bn.index, len(bn.twigs), bn.twigOffsetMax())
	}
	if len(bn.twigs) < 2 {
		return fmt.Errorf("branch at index %d has %d twigs", bn.index, len(bn.twigs))
	}
	if int(bn.index) <= parentIndex {
		return fmt.Errorf("branch index %d under branch index %d", bn.index, parentIndex)
	}
	for _, twig := range bn.twigs {
		if err := checkStructure(twig, int(bn.index)); err != nil {
			return err
		}
	}
	return nil
}

// trieShape returns the largest twig count of any branch and the number of
// nodes on the longest path from the root to a leaf.
func trieShape(n trieNode[int]) (maxTwigs, depth int) {
	bn, ok := n.(*branchNode[int])
	if !ok {
		if n == nil {
			return 0, 0
		}
		return 0, 1
	}
	maxTwigs = len(bn.twigs)
	for _, twig := range bn.twigs {
		w, d := trieShape(twig)
		maxTwigs = max(maxTwigs, w)
		depth = max(depth, d)
	}
	return maxTwigs, depth + 1
}

// checkModel compares the whole trie with the model: size, structure, iteration
// order, and Get and GetLessOrEqual for every stored key and every probe.
func checkModel(t *testing.T, tag string, tr *Trie[int], m trieModel, probes [][]byte) {
	t.Helper()
	keys := m.sortedKeys()
	if tr.Size() != len(keys) {
		t.Fatalf("%s: Size() = %d, want %d", tag, tr.Size(), len(keys))
	}
	if tr.root != nil {
		if err := checkStructure(tr.root, -1); err != nil {
			t.Fatalf("%s: %v", tag, err)
		}
	}

	it := tr.Iterator()
	for i, want := range keys {
		k, v, ok := it.Next()
		if !ok || string(k) != want || v != m[want] {
			t.Fatalf("%s: iterator item %d = %x, %v, %v; want %x, %v", tag, i, k, v, ok, want, m[want])
		}
	}
	if k, _, ok := it.Next(); ok {
		t.Fatalf("%s: iterator returned extra key %x", tag, k)
	}

	for _, key := range keys {
		checkLookup(t, tag, tr, m, keys, []byte(key))
	}
	for _, key := range probes {
		checkLookup(t, tag, tr, m, keys, key)
	}
}

func checkLookup(t *testing.T, tag string, tr *Trie[int], m trieModel, keys []string, key []byte) {
	t.Helper()
	want, wantFound := m[string(key)]
	val, found := tr.Get(key)
	if found != wantFound || (found && val != want) || (!found && val != 0) {
		t.Fatalf("%s: Get(%x) = %v, %v; want %v, %v", tag, key, val, found, want, wantFound)
	}

	k, v, exact, leFound := tr.GetLessOrEqual(key)
	i := sort.SearchStrings(keys, string(key)) // first stored key >= key
	switch {
	case wantFound:
		if !leFound || !exact || !bytes.Equal(k, key) || v != want {
			t.Fatalf("%s: GetLessOrEqual(%x) = %x, %v, %v, %v; want exact match", tag, key, k, v, exact, leFound)
		}
	case i == 0:
		if leFound || k != nil || v != 0 || exact {
			t.Fatalf("%s: GetLessOrEqual(%x) = %x, %v, %v, %v; want no key", tag, key, k, v, exact, leFound)
		}
	default:
		prev := keys[i-1]
		if !leFound || exact || string(k) != prev || v != m[prev] {
			t.Fatalf("%s: GetLessOrEqual(%x) = %x, %v, %v, %v; want %x, %v", tag, key, k, v, exact, leFound, prev, m[prev])
		}
	}
}

// checkCowFrontier verifies the copy-on-write rule of an open transaction.
// Going down from the root of the new trie, the first node on each path that
// also belongs to the base trie must be cow-marked. The nodes above it were
// created by the transaction and are changed in place; without the mark, the
// shared node would be changed in place as well.
func checkCowFrontier(t *testing.T, tag string, newTr, base *Trie[int]) {
	t.Helper()
	baseNodes := make(map[trieNode[int]]bool)
	var collect func(n trieNode[int])
	collect = func(n trieNode[int]) {
		if n == nil {
			return
		}
		baseNodes[n] = true
		if bn, ok := n.(*branchNode[int]); ok {
			for _, twig := range bn.twigs {
				collect(twig)
			}
		}
	}
	collect(base.root)

	unmarked := 0
	var walk func(n trieNode[int])
	walk = func(n trieNode[int]) {
		if n == nil {
			return
		}
		if baseNodes[n] {
			if !n.cowMarked() {
				unmarked++
			}
			return
		}
		if bn, ok := n.(*branchNode[int]); ok {
			for _, twig := range bn.twigs {
				walk(twig)
			}
		}
	}
	walk(newTr.root)
	if unmarked > 0 {
		t.Fatalf("%s: new trie reaches %d nodes of the base trie that are not cow-marked", tag, unmarked)
	}
}

// Test_ModelDirect checks Trie.Upsert, Trie.Delete, and the read operations
// on a trie that nothing else shares.
func Test_ModelDirect(t *testing.T) {
	const checks = 5
	for _, kg := range modelKeyGens {
		t.Run(kg.name, func(t *testing.T) {
			maxTwigs, maxDepth := 0, 0
			for seed := range kg.seeds {
				r := rand.New(rand.NewSource(int64(seed)))
				tr, m := New[int](), trieModel{}
				for step := range checks {
					tag := fmt.Sprintf("seed %d step %d", seed, step)
					applyRandomOps(t, tag, r, tr, m, kg.gen, kg.ops/checks)
					checkModel(t, tag, tr, m, randomKeys(r, kg.gen, 20))
					twigs, depth := trieShape(tr.root)
					maxTwigs, maxDepth = max(maxTwigs, twigs), max(maxDepth, depth)
				}
			}
			if maxTwigs < kg.minTwigs {
				t.Errorf("widest branch has %d twigs, generator should reach %d", maxTwigs, kg.minTwigs)
			}
			if maxDepth < kg.minDepth {
				t.Errorf("deepest path has %d nodes, generator should reach %d", maxDepth, kg.minDepth)
			}
		})
	}
}

type trieVersion struct {
	tr *Trie[int]
	m  trieModel
}

// Test_ModelCow runs one transaction after another, the way a single writer
// does, and keeps the last few committed versions the way readers do. After
// every commit or abort each kept version must still match its own model.
func Test_ModelCow(t *testing.T) {
	const (
		rounds       = 20
		maxTxnOps    = 20
		keepVersions = 4
	)
	for _, kg := range modelKeyGens {
		t.Run(kg.name, func(t *testing.T) {
			for seed := range kg.seeds {
				r := rand.New(rand.NewSource(int64(seed)))
				cur := trieVersion{New[int](), trieModel{}}
				applyRandomOps(t, fmt.Sprintf("seed %d build", seed), r, cur.tr, cur.m, kg.gen, kg.ops)

				var old []trieVersion
				for round := range rounds {
					tag := fmt.Sprintf("seed %d round %d", seed, round)
					probes := randomKeys(r, kg.gen, 20)

					tx := cur.tr.Txn()
					newM := cur.m.clone()
					applyRandomOps(t, tag, r, tx, newM, kg.gen, r.Intn(maxTxnOps+1))
					checkModel(t, tag+" open txn", tx.NewTrie(), newM, probes)
					checkCowFrontier(t, tag, tx.NewTrie(), cur.tr)

					if r.Intn(4) == 0 {
						if tr := tx.Abort(); tr != cur.tr {
							t.Fatalf("%s: Abort did not return the original trie", tag)
						}
					} else {
						old = append(old, cur)
						if len(old) > keepVersions {
							old = old[1:]
						}
						cur = trieVersion{tx.Commit(), newM}
					}

					checkModel(t, tag+" current", cur.tr, cur.m, probes)
					for i, v := range old {
						checkModel(t, fmt.Sprintf("%s old version %d", tag, i), v.tr, v.m, probes)
					}
				}
			}
		})
	}
}

// Test_ModelDirectAndTxn alternates direct changes and transactions on a trie
// with a single owner: every earlier version is dropped before the trie is
// changed directly.
func Test_ModelDirectAndTxn(t *testing.T) {
	const (
		rounds = 40
		maxOps = 15
	)
	kg := modelKeyGens[0]
	for seed := range kg.seeds {
		r := rand.New(rand.NewSource(int64(seed)))
		tr, m := New[int](), trieModel{}
		for round := range rounds {
			tag := fmt.Sprintf("seed %d round %d", seed, round)
			if r.Intn(2) == 0 {
				applyRandomOps(t, tag, r, tr, m, kg.gen, r.Intn(maxOps+1))
			} else {
				tx := tr.Txn()
				newM := m.clone()
				applyRandomOps(t, tag, r, tx, newM, kg.gen, r.Intn(maxOps+1))
				if r.Intn(3) == 0 {
					tr = tx.Abort()
				} else {
					tr, m = tx.Commit(), newM
				}
			}
			checkModel(t, tag, tr, m, randomKeys(r, kg.gen, 20))
		}
	}
}

// Fuzz_Ops decodes its input into a sequence of operations and then checks
// every version of the trie against its model.
//
// An operation is an opcode byte, a length byte, and 1 to 4 key bytes.
// Opcodes 0-3 upsert the key and 4-5 delete it. Opcode 6 opens a transaction,
// or commits the open one. Opcode 7 aborts the open transaction.
// Without an open transaction, writes go to the trie directly until the first
// commit. After that an older version exists, so such writes are skipped.
func Fuzz_Ops(f *testing.F) {
	f.Add([]byte("\x00\x00a\x00\x01ab\x00\x00b\x04\x00a"))
	f.Add([]byte("\x00\x00a\x00\x00b\x06\x00-\x01\x01ac\x04\x00a\x06\x00-\x06\x00-\x02\x00c\x07\x00-\x06\x00-\x05\x00b"))
	f.Add([]byte("\x06\x00-\x00\x02a\x00b\x00\x01a\x00\x04\x02a\x00b\x06\x00-\x06\x00-\x04\x01a\x00"))

	f.Fuzz(func(t *testing.T, data []byte) {
		cur := trieVersion{New[int](), trieModel{}}
		var (
			old   []trieVersion
			tx    *Txn[int]
			txM   trieModel
			probe [][]byte
		)
		for step := 0; len(data) >= 2; step++ {
			op, n := data[0]%8, int(data[1]%4)+1
			data = data[2:]
			if len(data) < n {
				break
			}
			key := data[:n]
			data = data[n:]
			probe = append(probe, key)

			switch {
			case op == 6 && tx == nil:
				tx, txM = cur.tr.Txn(), cur.m.clone()
			case op == 6:
				old = append(old, cur)
				cur, tx = trieVersion{tx.Commit(), txM}, nil
			case op == 7:
				if tx != nil {
					tx.Abort()
					tx = nil
				}
			case tx != nil && op < 4:
				tx.Upsert(key, step)
				txM[string(key)] = step
			case tx != nil:
				tx.Delete(key)
				delete(txM, string(key))
			case len(old) > 0:
				// Readers may hold an older version: no direct writes.
			case op < 4:
				cur.tr.Upsert(key, step)
				cur.m[string(key)] = step
			default:
				cur.tr.Delete(key)
				delete(cur.m, string(key))
			}
		}

		if tx != nil {
			checkModel(t, "open txn", tx.NewTrie(), txM, probe)
			checkCowFrontier(t, "open txn", tx.NewTrie(), cur.tr)
		}
		checkModel(t, "current", cur.tr, cur.m, probe)
		for i, v := range old {
			checkModel(t, fmt.Sprintf("old version %d", i), v.tr, v.m, probe)
		}
	})
}
