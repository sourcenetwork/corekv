# `regolith-baseline` — the native Rust lane

This crate is lane `rust` of the corekv FFI-overhead benchmark. It runs the thirteen
workloads from `PLAN-regolith.md`'s spec table against [`regolith`](https://crates.io/crates/regolith)
`0.1.4` directly - no C ABI, no cgo, no marshalling, no Go.

The two Go lanes (`go-ffi`, `badger`) run the same workloads through `corekv`. `go-ffi`
minus `rust` is **not** a measurement of the FFI boundary by itself: the two harnesses
report different statistics, build the engine with different profiles, and the Go
adapter does work besides crossing. See `../README.md` ("Reading the output") for the
full explanation and for `BenchmarkFFINoop`, the bare crossing cost.

It is a standalone crate with its own `Cargo.lock` and an empty `[workspace]`
table, so it is never pulled into a parent workspace.

## Running it

Full run (criterion defaults: 100 samples, 5 s per benchmark — long, because
`ScanAll/4KiB` moves 400 MB per iteration):

```sh
cd bench/rust-baseline
cargo bench
```

Fast smoke run (superseded by `baseline-results-full.txt`, see below):

```sh
cargo bench -- --sample-size 10 --measurement-time 2 2>&1 | tee baseline-results.txt
```

One workload, one value size:

```sh
cargo bench -- 'GetHit/64B'
```

Criterion writes HTML reports and raw samples to `target/criterion/`.

**Build profile.** `[profile.bench]` sets `lto = true` and `codegen-units = 1`,
matching the profile go-regolith's `ffi/Cargo.toml` builds the staticlib the Go
lane links with. Without it this crate compiles regolith with thin-local LTO
across 16 codegen units while the Go column's copy of the same engine gets fat
LTO and one unit, which handicaps this lane for a reason that has nothing to do
with the engine being measured.

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
| `BatchWriteNative` | 1 000 puts in one WriteBatch |
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
* **Options.** `shipping_options()`: `transaction_keys_inline = 4096`, everything
  else `Options::default()`, including `DurabilityMode::Eventual` (no fsync per
  write, which is badger's default too). The Go regolith lane opens with the
  same two settings (`bench/stores_regolith.go`).
* **Storage.** On disk, in a `tempfile::TempDir`. Not `MemEnv`; the badger lane
  is on disk, so this is too.
* **Transactions.** `OptimisticTransactionDb` with `IsolationLevel::Serializable`,
  matching what the Go lane ships and what badger's SSI validates.
* **Iteration.** `Snapshot::owned_iter()`, the cursor the FFI layer exposes —
  not the borrowing `Snapshot::iter()`. Reverse is `seek_to_last` + `prev`.
* **Prefill is never measured.** Databases are opened and filled (batched, never
  flushed) outside every measured closure. Neither Go lane can flush: go-regolith
  exposes no flush and badger has no public memtable flush.
* **Errors are fatal.** Every call `.expect(...)`s. `GetMiss` asserts it got
  `None`; `GetHit`/`Has` assert they got a hit; the scans assert their exact key
  counts. A workload that silently measured the wrong thing would abort instead.

## Deliberate shape choices

* `SeqWrite`/`RandWrite` reuse one database across iterations, overwriting the
  same 10 000 keys, because that is what a Go `b.N` loop over a fixed key set
  does. After the first iteration the engine is handling overwrites, on both
  sides equally.
* `BatchWrite` stays a 1000-key **transaction**, kept that way for continuity
  with every run that came before it.
* `BatchWriteNative` is the same 1000 keys through `Db::write(WriteBatch)`
  instead. `Db::write` consumes the batch, so this row rebuilds it inside the
  measurement, which the Go row's pooled-buffer adapter does not; the two are
  not identical in that one respect.
* Point-read workloads walk their shuffled permutation with a wrapping cursor
  across iterations, so all 100 000 keys get touched rather than only the first
  1 000 — otherwise the measurement would be of a cache-resident hot set.

## Measured baseline

These numbers were measured before this branch: at `[profile.bench] opt-level = 3`
with no cross-crate LTO, with keys formatted inside the timed closure on seven
rows, and with a flushing prefill. They are kept as a record of that
configuration. They are not comparable with a run of the harness as it stands
now, and there is no `BatchWriteNative` row because the row did not exist.
Re-measuring the whole table is a separate run.

`cargo bench` at criterion defaults (100 samples), darwin/arm64, rustc 1.95.0,
regolith 0.1.4, `Options::default()`. Criterion median iteration time divided by
the ops-per-iteration column.

| Workload | ops/iter | 64 B ns/op | 4 KiB ns/op |
|---|---:|---:|---:|
| `SeqWrite` | 10 000 | 2 512 | 9 845 |
| `RandWrite` | 10 000 | 3 533 | 12 137 |
| `GetHit` | 1 000 | 890 | 1 480 |
| `GetMiss` | 1 000 | 184 | 238 |
| `Has` | 1 000 | 795 | 1 026 |
| `ScanAll` | 100 000 | 73 | 280 |
| `ScanReverse` | 100 000 | 265 | 354 |
| `ScanPrefix` | 1 000 | 80 | 95 |
| `TxnWrite` | 100 | 1 699 | 8 503 |
| `TxnReadWrite` | 20 | 1 099 | 4 096 |
| `BatchWrite` | 1 000 | 1 850 | 6 834 |
| `ParallelMixed` | 4 000 | 1 057 | 2 061 |

`ParallelMixed` is aggregate: wall time per iteration over all 4 000 ops across
the 4 threads, which is what Go's `b.RunParallel` `ns/op` also reports.

### Result files

| File | What |
|---|---|
| `baseline-results-full.txt` | **Authoritative.** Full criterion run, 100 samples, all 24 benchmarks. |
| `baseline-per-op.md` | The table above, derived from that run. |
| `baseline-results.txt` | Superseded. Earlier smoke run at `--sample-size 10 --measurement-time 2`, kept for reference. |

Magnitudes agree between the two runs; the quick run reads 20-60 % slow on the
write and transaction workloads because ten samples do not outlast the first
compaction. Use the full run for the lane comparison.
