// Package qp implements a qp-trie (quadbit popcount trie): an ordered
// in-memory map from []byte keys to values.
//
// # Concurrency
//
// A Trie, Txn, and Iterator are not safe for concurrent use. All operations
// on a given trie, including transactions and iteration, must run from a
// single goroutine, or be coordinated by the caller.
//
// # Transactions
//
// The transaction model is one-at-a-time and copy-on-write. At most one
// transaction may be open in a snapshot family (a trie and the tries
// committed from it). Nested and concurrent transactions are not supported.
// While a transaction is open, every trie in the family is read-only except
// the transaction's working copy. After Commit or Abort a new transaction
// may be started.
package qp
