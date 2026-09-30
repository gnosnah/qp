# qp

Go qp-trie: an ordered in-memory map from `[]byte` keys to values.

**v1.** Not safe for concurrent use. Transactions are one-at-a-time.

## Features

- Get, Upsert, Delete
- Ordered iteration
- Copy-on-write transactions (one open at a time)
- Walk with optional filter

## Installation

```bash
go get github.com/gnosnah/qp
```

## Concurrency and transactions

A `Trie`, `Txn`, and `Iterator` must be used from a single goroutine, or with caller-provided synchronization.

A trie and every snapshot `Commit`ted from it form a family. Transaction rules:

- At most one transaction may be open in a family. A second `Txn()` on any trie in the family returns `ErrTxnOpen`.
- Nested transactions are not supported (`NewTrie().Txn()` returns `ErrTxnOpen`).
- Concurrent transactions are not supported.
- While a transaction is open, every trie in the family is read-only except the working copy. Writes return `ErrTrieFrozen`.
- After `Commit` or `Abort`, the family may be written again and may start a new transaction.
- Tries created by separate `New()` calls have separate families.

## v1 contract

These rules are part of the v1 API and will not change in a backward-incompatible way:

- **Single goroutine.** A `Trie`, `Txn`, and `Iterator` are not safe for concurrent use. The caller must serialize access.
- **One transaction per family.** Nested and concurrent transactions are not supported. While a transaction is open, only its working copy is writable.
- **Errors, not panics, for caller mistakes.** Empty or too-long keys return `ErrKeyEmpty` / `ErrKeyTooLong`. A second `Txn()` returns `ErrTxnOpen`. Writing a frozen snapshot returns `ErrTrieFrozen`. Using a finished `Txn` returns `ErrTxnFinished`. Internal invariant failures may still panic.
- **Keys and values.** Keys are `[]byte` and are copied on insert. Values are `any`. A key must not be empty and must not exceed `MaxKeyBytes` (32767).
- **Returned keys.** `Iterator.Next` and `GetLessOrEqual` may return slices owned by the trie; do not modify them. `Walk` copies keys into the result.
- **Walk.** `Walk` returns at most `max` pairs. The callback can only include or skip a pair; it cannot stop the walk early.
- **Compatibility.** Requires Go 1.21 or later. v1.x will not break these contracts; new features will be additive.

## Usage

### basic

```go
	tr := qp.New()

	keys := []string{"a", "b", "c", "f", "cef", "e", "cefy"}
	for _, key := range keys {
		if _, _, err := tr.Upsert([]byte(key), 1); err != nil {
			log.Fatal(err)
		}
	}

	val, found, err := tr.Get([]byte("a"))
	if err != nil {
		log.Fatal(err)
	}
	fmt.Printf("Get a, val: %v, found: %t \n", val, found)

	oldVal, found, err := tr.Delete([]byte("b"))
	if err != nil {
		log.Fatal(err)
	}
	fmt.Printf("Delete b, val: %v, found: %t \n", oldVal, found)
	fmt.Printf("size: %d \n", tr.Size())
```

### iteration

```go
	it := tr.Iterator()
	for {
		k, v, ok := it.Next()
		if !ok {
			break
		}
		fmt.Printf("key: %s, val: %v\n", k, v)
	}
```

The key returned by `Next` must not be modified.

### walk

```go
	result := tr.Walk(math.MaxInt, nil)
	fmt.Printf("result: %v \n", result)
```

### transaction

```go
	tx, err := tr.Txn()
	if err != nil {
		log.Fatal(err)
	}
	if _, _, err := tx.Upsert([]byte("b"), 2); err != nil {
		log.Fatal(err)
	}
	if _, _, err := tx.Upsert([]byte("x"), 3); err != nil {
		log.Fatal(err)
	}
	if _, _, err := tx.Delete([]byte("a")); err != nil {
		log.Fatal(err)
	}
	tr = tx.Commit() // or tx.Abort()
```

Assign the return value: `tr = tx.Commit()` or `tr = tx.Abort()`.

### customize

- onInsert

```go
	onInsert := func(newVal any) (finalVal any) {
		v := newVal.(int)
		return v * 10 // return newVal * 10
	}
	tr := qp.New(qp.WithOnInsert(onInsert))	
	for i := 1; i < 10; i++ {
		key := fmt.Sprintf("%d", i)
		val := i
		_, _, _ = tr.Upsert([]byte(string(key)), val)
	}	
	for i := 1; i < 10; i++ {
		key := fmt.Sprintf("%d", i)
		val, _, _ := tr.Get([]byte(string(key)))
		fmt.Printf("key: %s, value: %v \n", string(key), val)
	}

```

- onUpdate

```go
	onUpdate := func(newVal, oldVal any) (finalVal any) {
		v := newVal.(int)
		return v + oldVal.(int) // return newVal + oldVal
	}
	tr := qp.New(qp.WithOnUpdate(onUpdate))	
	for i := 1; i < 10; i++ {
		key := fmt.Sprintf("%d", i)
		val := i
		_, _, _ = tr.Upsert([]byte(string(key)), val)
	}	
	// update
	for i := 1; i < 10; i++ {
		key := fmt.Sprintf("%d", i)
		val := i
		_, _, _ = tr.Upsert([]byte(string(key)), val)
	}	
	for i := 1; i < 10; i++ {
		key := fmt.Sprintf("%d", i)
		val, _, _ := tr.Get([]byte(string(key)))
		fmt.Printf("key: %s, value: %v \n", string(key), val)
	}

```

- walk filter

```go
	tr := qp.New()
	kvs := []qp.KVPair{
		{Key: []byte("a"), Value: 1},
		{Key: []byte("ab"), Value: 2},
		{Key: []byte("b"), Value: 2},
		{Key: []byte("c"), Value: 3},
	}
	for _, d := range kvs {
		_, _, _ = tr.Upsert([]byte(d.Key), d.Value)
	}	
	// Return up to 10 pairs.
	max := 10
	// only reuturn key with 'a' prefix and value > 2
	f := func(key []byte, val any) bool { 
		v := val.(int)
		return bytes.HasPrefix(key, []byte("a")) && v >= 2
	}
	result := tr.Walk(max, f)
	fmt.Printf("result: %v \n", result)

```

## Benchmark
Ran some rough performance tests on virtual machine(4c8g), and the results were pretty impressive.

you can find benchmark tool [here](https://github.com/gnosnah/qp-bench)

gomap: golang1.24 builtin map(https://go.dev/blog/swisstable)  

```bash
$ cat /proc/cpuinfo
...
cpu MHz         : 2249.998
cache size      : 512 KB
...

$ ./bench.sh 3
benchmark iteration 1
Title  DataSize  Load(ms)  Insert(ms)  Get(ms)  Alloc(MB)  TotalAlloc(MB)  TotalSys(MB)
gomap  10000000  1334      7038        1608     1072       1281            1079
qp     10000000  620       3869        1309     1487       1598            1617

benchmark iteration 2
Title  DataSize  Load(ms)  Insert(ms)  Get(ms)  Alloc(MB)  TotalAlloc(MB)  TotalSys(MB)
gomap  10000000  619       6278        1897     1079       1281            1087
qp     10000000  665       3638        1318     1489       1598            1613

benchmark iteration 3
Title  DataSize  Load(ms)  Insert(ms)  Get(ms)  Alloc(MB)  TotalAlloc(MB)  TotalSys(MB)
gomap  10000000  646       6116        1692     1052       1281            1059
qp     10000000  621       3435        1288     1484       1598            1634

```


## Reference
- https://github.com/fanf2/qp
- https://gitlab.nic.cz/knot/knot-dns/-/tree/v3.3.3/src/contrib/qp-trie?ref_type=tags
- https://github.com/tatsushid/go-critbit

## ❤
qp-trie is awesome, thanks [@fanf2](https://github.com/fanf2)
