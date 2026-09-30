# Changelog

## 1.0.0

First stable release.

- One transaction at a time per snapshot family; nested and concurrent transactions are not supported.
- While a transaction is open, sibling and ancestor snapshots are read-only.
- User-facing failures return exported errors instead of panicking.
- Requires Go 1.21 or later.
