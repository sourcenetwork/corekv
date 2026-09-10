# `regolith-baseline` — the native Rust lane

This crate is lane `rust` of the corekv FFI-overhead benchmark. It runs the twelve
workloads from `PLAN-regolith.md`'s spec table against [`regolith`](https://crates.io/crates/regolith)
`0.1.4` directly — no C ABI, no cgo, no marshalling, no Go.

Its numbers are the **denominator**. The two Go lanes (`go-ffi`, `badger`) run the
same workloads through `corekv`:

```
go-ffi  -  rust    =  FFI + marshalling + copy cost
go-ffi  vs badger  =  the actual decision
```

It is a standalone crate with its own `Cargo.lock` and an empty `[workspace]`
table, so it is never pulled into a parent workspace.

## Running it

Full run (criterion defaults: 100 samples, 5 s per benchmark — long, because
`ScanAll/4KiB` moves 400 MB per iteration):

```sh
cd bench/rust-baseline
cargo bench
```

Fast run, what `baseline-results.txt` was produced with:

```sh
cargo bench -- --sample-size 10 --measurement-time 2 2>&1 | tee baseline-results.txt
```

One workload, one value size:

```sh
cargo bench -- 'GetHit/64B'
```

Criterion writes HTML reports and raw samples to `target/criterion/`.

## Reading the numbers

Criterion reports **time per iteration**, and an iteration is not one store
operation. Every benchmark declares `Throughput::Elements(n)`, where `n` is the
number of store operations in one iteration:

| Workload | ops per iteration (`n`) |
|---|---|
| `SeqWrite`, `RandWrite` | 10 000 puts |
| `GetHit`, `GetMiss`, `Has` | 1 000 point ops |
| `ScanAll`, `ScanReverse` | 100 000 cursor steps |
| `ScanPrefix` | 1 000 cursor steps |
| `TxnWrite` | 100 puts (+ 1 commit) |
| `TxnReadWrite` | 20 ops — 10 gets + 10 puts (+ 1 commit) |
| `BatchWrite` | 1 000 puts (+ 1 commit) |
| `ParallelMixed` | 4 000 — 4 threads × 1 000 ops, 90 % get / 10 % put |

So:

```
ns per operation  =  iteration time (ns) / n
```

Criterion prints this for you as the `thrpt` line in `Melem/s`; the per-op
figure is `1000 / (Melem/s)` nanoseconds. The Go lanes report `ns/op` where
`op` is one key, so those two figures compare directly.

Caveat on the transaction workloads: the commit is amortised over the `n` puts,
so a smaller `n` carries a larger share of fixed commit cost per operation.
`TxnWrite` (100) and `BatchWrite` (1000) differ mostly for that reason, and
`TxnReadWrite` (20) is dominated by it. Compare each workload lane-to-lane, not
workload-to-workload.

## Fidelity constraints

The comparison is only worth something if both sides do literally the same work.
What is pinned:

* **Keys.** `format!("key:{i:012}")` — 16 bytes, byte-identical to Go's
  `fmt.Sprintf("key:%012d", i)`.
* **Access order.** Random orders come from a xorshift64\* generator seeded with
  42 and a descending Fisher-Yates shuffle. Both are written out in full, with
  the Go translation, in the doc comments on `Rng` and `shuffled()` in
  `benches/workloads.rs`. **The Go lane must copy that code verbatim.**
* **Options.** `Options::default()` and nothing else. Default
  `DurabilityMode::Eventual` — no fsync per write, which is also badger's
  default. Neither engine is tuned.
* **Storage.** On disk, in a `tempfile::TempDir`. Not `MemEnv`; the badger lane
  is on disk, so this is too.
* **Transactions.** `OptimisticTransactionDb` with
  `IsolationLevel::SnapshotIsolation`, which is what the FFI layer uses.
* **Iteration.** `Snapshot::owned_iter()`, the cursor the FFI layer exposes —
  not the borrowing `Snapshot::iter()`. Reverse is `seek_to_last` + `prev`.
* **Prefill is never measured.** Databases are opened and filled (batched, then
  flushed) outside every measured closure.
* **Errors are fatal.** Every call `.expect(...)`s. `GetMiss` asserts it got
  `None`; `GetHit`/`Has` assert they got a hit; the scans assert their exact key
  counts. A workload that silently measured the wrong thing would abort instead.

## Deliberate shape choices

* `SeqWrite`/`RandWrite` reuse one database across iterations, overwriting the
  same 10 000 keys, because that is what a Go `b.N` loop over a fixed key set
  does. After the first iteration the engine is handling overwrites, on both
  sides equally.
* `BatchWrite` is a 1000-key **transaction**, not `Db::write(WriteBatch)`.
  corekv's `TxnStore` has no batch primitive, so a native `WriteBatch` number
  would have no Go counterpart to be compared with.
* Point-read workloads walk their shuffled permutation with a wrapping cursor
  across iterations, so all 100 000 keys get touched rather than only the first
  1 000 — otherwise the measurement would be of a cache-resident hot set.

## Measured baseline

`cargo bench -- --sample-size 10 --measurement-time 2`, darwin/arm64, rustc 1.95.0,
regolith 0.1.4, `Options::default()`. Criterion median iteration time divided by
the ops-per-iteration column. Full criterion output in `baseline-results.txt`.

| Workload | ops/iter | 64 B ns/op | 4 KiB ns/op |
|---|---:|---:|---:|
| `SeqWrite` | 10 000 | 3 963 | 8 873 |
| `RandWrite` | 10 000 | 3 695 | 9 655 |
| `GetHit` | 1 000 | 960 | 2 069 |
| `GetMiss` | 1 000 | 174 | 245 |
| `Has` | 1 000 | 793 | 1 228 |
| `ScanAll` | 100 000 | 72 | 245 |
| `ScanReverse` | 100 000 | 261 | 321 |
| `ScanPrefix` | 1 000 | 87 | 102 |
| `TxnWrite` | 100 | 1 293 | 5 714 |
| `TxnReadWrite` | 20 | 809 | 3 481 |
| `BatchWrite` | 1 000 | 1 357 | 6 854 |
| `ParallelMixed` | 4 000 | 1 221 | 3 479 |

`ParallelMixed` is aggregate: wall time per iteration over all 4 000 ops across
the 4 threads, which is what Go's `b.RunParallel` `ns/op` also reports.

A 10-sample run has wide confidence intervals — `SeqWrite/64B` spans 31–50 ms.
Re-run at criterion defaults before publishing a final comparison; these figures
are for sanity-checking magnitudes and for the FFI delta, which is large enough
to survive the noise.
