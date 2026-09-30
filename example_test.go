package qp_test

import (
	"fmt"

	"github.com/gnosnah/qp"
)

func Example() {
	tr := qp.New()
	_, _, err := tr.Upsert([]byte("b"), 1)
	if err != nil {
		panic(err)
	}
	_, _, err = tr.Upsert([]byte("a"), 2)
	if err != nil {
		panic(err)
	}

	it := tr.Iterator()
	for {
		k, v, ok := it.Next()
		if !ok {
			break
		}
		fmt.Printf("%s %v\n", k, v)
	}
	// Output:
	// a 2
	// b 1
}

func ExampleTrie_Txn() {
	tr := qp.New()
	_, _, _ = tr.Upsert([]byte("a"), 1)

	tx, err := tr.Txn()
	if err != nil {
		panic(err)
	}
	_, _, _ = tx.Upsert([]byte("b"), 2)
	tr = tx.Commit()

	v, found, _ := tr.Get([]byte("b"))
	fmt.Println(found, v)
	// Output:
	// true 2
}

func ExampleTrie_Walk() {
	tr := qp.New()
	_, _, _ = tr.Upsert([]byte("a"), 1)
	_, _, _ = tr.Upsert([]byte("b"), 2)

	for _, p := range tr.Walk(10, nil) {
		fmt.Printf("%s %v\n", p.Key, p.Value)
	}
	// Output:
	// a 1
	// b 2
}
