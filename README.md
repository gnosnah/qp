# qp

golang version of qp trie

## Features

- Fast Get, Upsert(insert/update)
- Ordered iteration
- Transaction(Copy-on-write)
- Trie walk

## Installation

```bash
go get -u github.com/gnosnah/qp
```

## Usage

### basic 

- new/get/upsert/delete/size

```go
	tr := qp.New()

	keys := []string{"a", "b", "c", "f", "cef", "e", "cefy"}
	for _, key := range keys {
		tr.Upsert([]byte(key), 1)
	}

	size := tr.Size()
	fmt.Printf("after Upsert, trie size: %d \n", size)

	val, found := tr.Get([]byte("a"))
	fmt.Printf("Get a, val: %v, found: %t \n", val, found)

	oldVal, found := tr.Delete([]byte("b"))
	fmt.Printf("Delete b, val: %v, found: %t \n", oldVal, found)

	size = tr.Size()
	fmt.Printf("after Delete, trie size: %d \n", size)
```

- iteration

```go 
	tr := qp.New()
	keys := []string{"a", "b", "c", "f", "cef", "e", "cefy"}
	for _, key := range keys {
		tr.Upsert([]byte(key), 1)
	}	
	it := tr.Iterator()
	for {
		k, v, ok := it.Next()
		if !ok {
			break
		}
		fmt.Printf("key: %v, val: %v\n", k,v)
	}

```

- walk

``` go
	tr := qp.New()
	keys := []string{"a", "b", "c", "f", "cef", "e", "cefy"}
	for _, key := range keys {
		tr.Upsert([]byte(key), 1)
	}	
	max := math.MaxInt
	result := tr.Walk(max, nil)
	fmt.Printf("result: %v \n", result)

```

- transaction

```go
	tr := qp.New()
	keys := []string{"a", "b", "c", "f", "cef", "e", "cefy"}
	for _, key := range keys {
		tr.Upsert([]byte(key), 1)
	}

	tx := tr.Txn()
	tx.Upsert([]byte("b"), 2)
	tx.Upsert([]byte("x"), 3)
	tx.Delete([]byte("a"))
	// Call Commit or Abort once, and keep the returned trie.
	// Do not use tx after this. Reassigning tr drops the previous trie,
	// so later Upsert and Delete on tr are safe.
	tr = tx.Commit() // or tx.Abort()
	result := tr.Walk(10, nil)
	for _, d := range result {
		fmt.Printf("key: %s, value: %v \n", string(d.Key), d.Value)
	}

```

Notes:

- Assign the return value: `tr = tx.Commit()` or `tr = tx.Abort()`.
- `Commit` and `Abort` are mutually exclusive, and each may be called only once. Do not use the `Txn` after either call (`Get`, `Upsert`, `Delete`, `OldTrie`, `NewTrie`). Keep using the returned `*Trie`.
- Do not modify the trie directly while a transaction is open.
- A transaction shares unmodified nodes with the original trie, `OldTrie()`, `NewTrie()`, and the trie returned by `Commit` or `Abort`. Those tries may be read together. While any of them is still in use, modify the data only through a new `Txn()`. `Trie.Upsert` and `Trie.Delete` are safe only after every other reference has been dropped.
- Do not run concurrent transactions on the same trie.
- Do not start a nested transaction from `tx.NewTrie()`.

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
		tr.Upsert([]byte(string(key)), val)
	}	
	for i := 1; i < 10; i++ {
		key := fmt.Sprintf("%d", i)
		val, _ := tr.Get([]byte(string(key)))
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
		val, _ := tr.Get([]byte(string(key)))
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
		tr.Upsert([]byte(d.Key), d.Value)
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

gomap: golang1.26.6 builtin map

```bash
$ cat /proc/cpuinfo
...
cpu MHz         : 2249.998
cache size      : 512 KB
...

❯ ./bench.sh 2
benchmark iteration 1
benchmark: gomap
benchmark: qp
Title  DataSize  Load(ms)  Insert(ms)  Get(ms)  Alloc(MB)  TotalAlloc(MB)  TotalSys(MB)
gomap  10000000  1323      7557        5130     641        1281            1095
qp     10000000  1150      6037        2333     1497       1598            1622
benchmark iteration 2
benchmark: gomap
benchmark: qp
Title  DataSize  Load(ms)  Insert(ms)  Get(ms)  Alloc(MB)  TotalAlloc(MB)  TotalSys(MB)
gomap  10000000  1189      7030        4104     649        1281            1103
qp     10000000  1284      5793        2142     1498       1598            1619

```


## Limits

The key must not be nil and must not exceed 32767 bytes.

## Reference
- https://github.com/fanf2/qp
- https://gitlab.nic.cz/knot/knot-dns/-/tree/v3.3.3/src/contrib/qp-trie?ref_type=tags
- https://github.com/tatsushid/go-critbit

## ❤
qp-trie is awesome, thanks [@fanf2](https://github.com/fanf2)
