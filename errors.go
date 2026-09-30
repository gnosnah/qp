package qp

import (
	"errors"
	"fmt"
	"math"
)

// MaxKeyBytes is the maximum length of a key in bytes.
const MaxKeyBytes = math.MaxUint16 >> 1

var (
	// ErrKeyEmpty is returned when a key is nil or has length 0.
	ErrKeyEmpty = errors.New("empty key")
	// ErrKeyTooLong is returned when a key exceeds MaxKeyBytes.
	ErrKeyTooLong = fmt.Errorf("max key length is %d bytes", MaxKeyBytes)
	// ErrTxnOpen is returned when Txn is called while a transaction is already
	// open on this trie or on another snapshot in the same family.
	ErrTxnOpen = errors.New("a transaction is already open")
	// ErrTxnFinished is returned when a committed or aborted transaction is used.
	ErrTxnFinished = errors.New("transaction is already committed or aborted")
	// ErrTrieFrozen is returned when writing a trie that is the origin of an
	// open transaction, or another snapshot in the same family while that
	// transaction is open.
	ErrTrieFrozen = errors.New("trie cannot be modified while a transaction is open")
)

var errInternal = errors.New("internal error")

func checkKey(key []byte) error {
	if len(key) == 0 {
		return ErrKeyEmpty
	}
	if len(key) > MaxKeyBytes {
		return ErrKeyTooLong
	}
	return nil
}
