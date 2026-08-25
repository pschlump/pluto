# AGENTS.md — Pluto

## Project Overview

Pluto is a **library of classic data structures written with Go generics**
(Go 1.23+; iterators use range-over-func). It is a pure library — there is no
binary, server, or application entry point anywhere in the module.

- Module path: `github.com/pschlump/pluto` (declared in `go.mod`, Go 1.26.5)
- License: BSD 3-Clause (`LICENSE`)
- Author: Philip Schlump

Every data structure lives in its own subdirectory, and each subdirectory is
one Go package of the module. Most structures come in two flavors:

- a plain (single-threaded) package, e.g. `avl_tree/`
- a thread-safe twin with an **identical API** in a directory ending in
  `_ts`, e.g. `avl_tree_ts/`, guarded by an internal `sync.RWMutex`

The single source of truth for what each package does, its complexity
guarantees, and its pitfalls is **`README.adoc`** — read it before working on
any package.

### Package map

| Category | Packages |
|---|---|
| Linear | `stack`, `stack_sll_ts`, `sll`, `sll_ts`, `simple_sll`, `dll`, `dll_ts`, `queue`, `queue_ts`, `queue_dll_ts`, `dqueue_ts` |
| Trees | `bst`, `binary_tree`, `binary_tree_ts`, `avl_tree`, `avl_tree_ts`, `rb_tree`, `rb_tree_ts` |
| Hash tables | `hash_tab`, `hash_tab_dll`, `hash_tab_bt`, `hash_tab_bt_ts`, `hash_grow`, `hash_grow_ts` |
| Skip lists | `skip_list`, `skip_list_ts`, `skip_list_dll`, `skip_list_dll_ts` |
| Heaps / PQ | `heap`, `heap_ts`, `heap_sort`, `priority_queue`, `priority_queue_ts` |
| Support | `comparable`, `g_lib`, `iface_list` |
| Not library code | `note/` (git-ignored) holds all non-library content: `note/ex1` (teaching example), `note/article` (AsciiDoc article), `note/y1`, `note/iter0`, `note/it` (old iterator experiments), plus reference papers/notes |

Dependencies (`go.mod`): `github.com/pschlump/HashStr`, `MiscLib`, and
`dbgo`.

## Build and Test Commands

Use the root `Makefile`:

- `make` / `make build` — `go build ./...`
- `make vet` — `go vet ./...`
- `make test` — `go test ./... -count=1`
- `make race` — all tests with the race detector
- `make cover` — per-package coverage report
- `make bench` — `go test -run='^$' -bench=. -benchmem ./...`
- `make lint` — `golangci-lint run ./...`
- `make tidy` — `go mod tidy`

To work on a single package, `cd` into its directory; each package also has
its own trivial `Makefile` with `all` (`go build`) and `test` (`go test`)
targets. As of the last verification, `go build ./...`, `go vet ./...`, and
the full package test suites pass.

There is no CI configuration in the repo. `sync-to-victoria.sh` is the
author's personal rsync script for copying the tree to another machine — it
is not a deployment process and should not be run or relied on.

## Code Organization and Conventions

- **One package per directory.** Files are consistently named after the
  structure: `avl_tree/avl_tree.go`, `avl_tree/avl_tree_test.go`, plus
  `iter.go` for iterator code where applicable.
- **Constraints.** Ordered structures (trees, heaps, skip lists) require the
  element type to implement `comparable.Comparable` (`Compare(b) int`). Hash
  tables and linked lists require `comparable.Equality` (`IsEqual(b) bool`).
  Slice-based containers (`stack`, `queue`) accept plain `T any`. Note the
  known ergonomic wart: these interface methods take the interface type, so
  implementations need a type assertion.
- **Pointers vs. values.** Linked structures and the `avl_tree`/`binary_tree`
  families store and return `*T`; `bst` and `rb_tree` use a value-based API
  (`Insert(item T)`, iterators yield `T`). Match the style of the package you
  are editing.
- **Zero value usable.** Most structures work without a constructor; keep
  that property when adding fields.
- **Uniform tree API.** All trees share: `Insert` (duplicates replace),
  `Search`, `Delete`, `FindMin`/`FindMax`, `DeleteAtHead`/`DeleteAtTail`,
  `IsEmpty`/`Length`, `Truncate`, `Depth`, `Dump`, and `All`/`Backward`
  range-over-func iterators.
- **Copyright header.** Source files carry a
  `Copyright (C) Philip Schlump, 2012-2021. BSD 3 Clause Licensed.` comment
  block; new files should follow suit.
- **Comments and docs are in English.** All READMEs are AsciiDoc
  (`README.adoc`); follow the style of `dll/README.adoc`. Several directories
  contain informal `note.1` / `todo.1` working notes — leave them alone.
- **A plain package and its `_ts` twin must keep identical APIs.** The plain
  variants expose no-op `Lock`/`Unlock` methods specifically so code written
  against the `_ts` variant compiles unchanged. When you change one, change
  the other.

## Thread-Safety Rules (`_ts` packages)

A package is **not** thread-safe unless its directory ends in `_ts`. When
touching a `_ts` package, preserve these established semantics:

- Every operation is guarded by an internal `sync.RWMutex`.
- `All`/`Backward` iterators generally walk a **snapshot** taken under the
  read lock (safe to mutate while iterating). Exceptions: `queue_dll_ts` and
  `dqueue_ts` iterate live; `skip_list_dll_ts` holds the read lock for the
  whole iteration.
- `Walk`-style callbacks run **under the read lock** and must not call back
  into the structure (deadlock).
- Some packages return copies of stored items (`rb_tree_ts`, `skip_list_ts`,
  `skip_list_dll_ts`); others return live `*T` aliases that callers must
  treat as read-only. Do not change which behavior a package has without
  updating `README.adoc`.
- Thread-safe packages have real goroutine concurrency tests (e.g.
  `dll_ts/dll_goroute_test.go`, `rb_tree_ts`); run `make race` after changes.

## Testing Instructions

- Tests use only the standard library `testing` package — no external test
  frameworks or assertion libraries. Style is plain table-free tests with
  `t.Errorf`/`t.Fatalf` (see `stack/stack_test.go` for the idiom).
- Every package has a `*_test.go` next to the implementation; add tests
  there when changing behavior.
- Run `make test` before considering work done; run `make race` when you
  touched any `_ts` package.

## Known Pitfalls (documented in README.adoc)

- `iface_list` interface method names do not exactly match every concrete
  API.
- `binary_tree`/`bst` degenerate to O(n) on sorted input; `avl_tree` and
  `rb_tree` are the guaranteed-O(log n) options.
- `hash_tab`/`hash_tab_dll` allow duplicate keys; `hash_grow` and
  `hash_tab_bt` replace on equal insert.

## Security Considerations

- This is a data-structure library with no network, file, or process I/O in
  library code, and no secrets or credentials belong in it.
- Do not run or "fix" `sync-to-victoria.sh` — it rsyncs the tree to the
  author's personal machines over ssh.
- `note/` contains downloaded third-party PDFs and reference material; treat
  it as read-only input, never execute or modify it.

## Keeping This File Current

If you add a package, change an API contract, or alter the build/test
workflow, update both `README.adoc` (the user-facing catalog) and this file.
