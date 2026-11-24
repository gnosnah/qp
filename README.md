# qp

Go implementation of a [qp-trie](https://github.com/fanf2/qp) (quelques-bits
popcount patricia trie): an ordered map from byte-string keys to values.

A qp-trie is like a crit-bit / patricia trie, but each branch indexes a few bits
at a time and compresses unused children with a popcount bitmap. This port uses
4-bit (nibble) branches, supports arbitrary binary keys (including embedded
NULs and proper prefixes), and adds copy-on-write transactions for cheap
snapshots. It is inspired by [Tony Finch's qp](https://github.com/fanf2/qp) and
the qp-trie used in [Knot DNS](https://gitlab.nic.cz/knot/knot-dns/-/tree/v3.3.3/src/contrib/qp-trie?ref_type=tags).

## Features

- Generic `Trie[V]` with `Get`, `Upsert`, and `Delete`
- Ordered iteration and filtered `Walk`
- Predecessor query via `GetLessOrEqual`
- Copy-on-write transactions (`Txn`) for versioned updates
- Inserted keys are copied; callers may reuse their buffers after `Upsert`

## Installation

Requires Go 1.24 or later.

```bash
go get -u github.com/gnosnah/qp
```

## Usage

Create with an explicit value type (`qp.New[int]()`), or let `WithOnInsert` /
`WithOnUpdate` infer it. Misses return the zero value of `V` with
`found == false`. Do not copy `Trie`, `Txn`, or `Iterator` by value after
creation — pass pointers.

### Get, Upsert, Delete

```go
tr := qp.New[int]()
tr.Upsert([]byte("apple"), 1)
tr.Upsert([]byte("apricot"), 2)
tr.Upsert([]byte("banana"), 3)

v, found := tr.Get([]byte("apple"))    // 1, true
old, ok := tr.Delete([]byte("banana")) // 3, true
n := tr.Size()                         // 2
```

### Iteration, Walk, GetLessOrEqual

```go
it := tr.Iterator()
for {
	k, v, ok := it.Next()
	if !ok {
		break
	}
	fmt.Println(string(k), v)
}

// Up to 10 pairs; nil filter accepts every key.
pairs := tr.Walk(10, func(key []byte, val int) bool {
	return bytes.HasPrefix(key, []byte("ap"))
})

k, v, exact, found := tr.GetLessOrEqual([]byte("apricot"))
// k == "apricot", exact == true, found == true
```

Keys from `Iterator.Next`, `Walk`, and `GetLessOrEqual` belong to the trie —
do not modify them. `GetLessOrEqual` does not retain the query buffer.

### Transaction

```go
tx := tr.Txn()
tx.Upsert([]byte("cherry"), 4)
tx.Delete([]byte("apple"))
tr = tx.Commit() // or tx.Abort(); keep the returned *Trie
```

Rules:

- Assign `tr = tx.Commit()` or `tr = tx.Abort()`. Each may be called only once; afterwards every `Txn` method panics with `transaction is not open`.
- Do not mutate a trie directly while a transaction on it is open, and do not open a nested `Txn` from `tx.NewTrie()`.
- Only one transaction may be open at a time across a trie and every version that shares nodes with it (`OldTrie`, `NewTrie`, and earlier `Commit`/`Abort` results).
- Shared versions may be read together. While any shared version is still live, write only through a new `Txn()`. `Trie.Upsert` / `Trie.Delete` are safe only after every other reference has been dropped.
- Values are not copied across versions — treat stored values as immutable; `onUpdate` must return a new value, not mutate `oldVal`.

### Concurrency

No internal locking: one writer, any number of readers.

- Readers may call `Get`, `GetLessOrEqual`, `Iterator`, `Walk`, and `Size` concurrently, including while a transaction is open or after a newer version is committed.
- Writes (`Txn`, `Upsert`, `Delete`, `Commit`, `Abort`) must be single-threaded.
- Publish `Commit`'s result with something like `atomic.Pointer`. Do not share `tx.NewTrie()` before `Commit`. Once readers exist, mutate only through `Txn()`.

```go
var zone atomic.Pointer[qp.Trie[int]]
zone.Store(qp.New[int]())

tr := zone.Load()                // reader
v, found := tr.Get([]byte("apple"))

tx := zone.Load().Txn()          // single writer
tx.Upsert([]byte("apple"), 1)
zone.Store(tx.Commit())
```

### Customize

```go
tr := qp.New(qp.WithOnInsert(func(v int) int { return v * 10 }))
tr.Upsert([]byte("a"), 3) // stores 30

tr = qp.New(qp.WithOnUpdate(func(newVal, oldVal int) int { return newVal + oldVal }))
tr.Upsert([]byte("a"), 1)
tr.Upsert([]byte("a"), 2) // stores 3
```

## Benchmark

Compared with Go map, [Google B-tree](https://github.com/google/btree) `v1.0.0`,
and [HashiCorp immutable radix](https://github.com/hashicorp/go-immutable-radix)
`v2.1.0`, using 10,000 unique random 16-byte keys. Figures are medians of three
100 ms runs on one CPU (Go 1.27.3, Xeon Platinum 8336C); lower is better.

| Operation | QP | Go map | B-tree | Radix |
|---|---:|---:|---:|---:|
| Build, ns/key | 358.7 | 100.2 | 518.9 | 1,019.1 |
| Get hit, ns/op | 75.82 | 26.70 | 378.0 | 260.5 |
| Ordered scan, ns/key | 49.81 | 240.3 | 5.649 | 12.56 |
| Snapshot update of 16 keys, us/op | 18.77 | 302.14 | 27.38 | 40.99 |

Go map led on build and lookup. QP looked up keys faster than B-tree and radix
and led on snapshot updates. B-tree led on ordered scans. Map's scan sorts its
keys, and its snapshot update clones the map. In a separate 100,000-key
retained-heap test, QP used 101.86 B/key versus 50.95 B/key for map. B-tree and
radix borrow input keys, so their memory use is not directly comparable with QP
or map.

## Limits

- The key must not be empty (nil or zero length) and must not exceed 32767 bytes. `Get`, `Upsert`, and `Delete` (on both `Trie` and `Txn`) and `GetLessOrEqual` panic on such a key.
- Create a trie with `qp.New`. The zero value `qp.Trie[V]{}` is not usable.
- An iterator is valid only while its trie is not modified. `Trie.Upsert` and `Trie.Delete` on that trie invalidate it, and so do `Txn.Upsert` and `Txn.Delete` for an iterator over `tx.NewTrie()`. Afterwards `Next` may repeat keys, skip keys, or panic. A transaction does not modify the trie it was started from, so an iterator over that trie stays valid.
- `onInsert` and `onUpdate` handlers and the `Walk` filter must not modify the trie or transaction that calls them.

## Reference

- [fanf2/qp](https://github.com/fanf2/qp) — original qp-trie / crit-bit research and C code by Tony Finch
- [Knot DNS qp-trie](https://gitlab.nic.cz/knot/knot-dns/-/tree/v3.3.3/src/contrib/qp-trie?ref_type=tags) — production C implementation used in Knot DNS
- [tatsushid/go-critbit](https://github.com/tatsushid/go-critbit) — related Go crit-bit tree

## License

MIT. See [LICENSE](LICENSE).

## Thanks

qp-trie is awesome — thanks [@fanf2](https://github.com/fanf2).
