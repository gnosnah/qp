package qp_test

import (
	"fmt"

	"github.com/gnosnah/qp"
)

func ExampleNew() {
	tr := qp.New[int]()
	tr.Upsert([]byte("apple"), 1)
	tr.Upsert([]byte("apricot"), 2)
	tr.Upsert([]byte("banana"), 3)

	v, found := tr.Get([]byte("apple"))
	fmt.Println(v, found)

	it := tr.Iterator()
	for {
		k, v, ok := it.Next()
		if !ok {
			break
		}
		fmt.Println(string(k), v)
	}

	k, v, exact, found := tr.GetLessOrEqual([]byte("apricot"))
	fmt.Println(string(k), v, exact, found)

	// Output:
	// 1 true
	// apple 1
	// apricot 2
	// banana 3
	// apricot 2 true true
}

func ExampleTrie_Txn() {
	tr := qp.New[int]()
	tr.Upsert([]byte("apple"), 1)
	tr.Upsert([]byte("banana"), 2)

	tx := tr.Txn()
	tx.Upsert([]byte("cherry"), 3)
	tx.Delete([]byte("apple"))
	tr = tx.Commit()

	it := tr.Iterator()
	for {
		k, v, ok := it.Next()
		if !ok {
			break
		}
		fmt.Println(string(k), v)
	}

	// Output:
	// banana 2
	// cherry 3
}
