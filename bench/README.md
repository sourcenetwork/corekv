# corekv bench

Benchmark harness for comparing corekv store implementations, built to isolate the cost of the
cgo/FFI boundary for the regolith store. See `../PLAN-regolith.md` for the plan and the workload
spec this implements.

Three lanes are intended:

| lane | what it is | status |
|------|-----------|--------|
| `memory` | `corekv/memory`, in-process | wired up (sanity control, should win everything) |
| `badger` | `corekv/badger`, **on disk**, default options | wired up (the incumbent) |
| `regolith` | `corekv/regolith` over cgo, on disk, default options | seam only, behind `-tags regolith` |
| `rust` | regolith called natively from Rust (criterion) | separate crate, `rust-baseline/` |

## Running

```sh
./run.sh                 # badger + memory
./run.sh -tags regolith  # also the regolith lane
go test -run '^$' -bench GetHit -benchtime 3s ./...   # one workload
```

`run.sh` uses `-bench . -benchtime 3s -count 5 -run '^$'` and tees to `results-<stamp>.txt`.
Extra arguments are passed through to `go test`.

The full suite is not cheap: it prefills 100k keys per (lane, value size) and the write workloads
move 10k values per iteration, so at the 4 KiB size a `-benchtime 3s` run churns a few GB through
`b.TempDir()` (removed afterwards) and holds ~400 MB resident for the memory lane. Use
`-benchtime 1x` while iterating on the harness itself (the whole suite takes ~10 min that way; a
full `-benchtime 3s -count 5` run is on the order of an hour).

## Naming

`Benchmark<Workload>/<lane>/v<valueSize>`, e.g.:

```
BenchmarkGetHit/badger/v64
BenchmarkGetHit/badger/v4096
BenchmarkScanAll/memory/v64
BenchmarkFFINoop              # no lane/size; skips without -tags regolith
BenchmarkTxnContendedHot8/badger/v64    # contention level in the workload name
```

Workloads: `SeqWrite RandWrite GetHit GetMiss Has ScanAll ScanAllAppend ScanAllBorrow ScanReverse
ScanPrefix TxnWrite TxnReadWrite BatchWrite ParallelMixed` — one top-level `Benchmark*` per row of
the spec table — plus `TxnContendedHot8` and `TxnContendedHot4096`, which are not in the spec table
(see "Contended transactions" below).

`ScanAllAppend` and `ScanAllBorrow` are `ScanAll` reading each value through the optional
`corekv.ValueAppender` / `corekv.ValueBorrower` interfaces, falling back to `Value` for
iterators that do not implement them. They share `ScanAll`'s `opsPerIter`, so the three are
directly comparable: `ScanAll` allocates a buffer and copies into it, `ScanAllAppend` copies into
a re-used buffer, `ScanAllBorrow` does neither. They have no Rust counterpart — they measure a
property of the Go interface, not of the engine — and so are outside the fidelity comparison.

## Reading the output

Most workloads do many store operations per `b.N` iteration (10k `Set`s for `SeqWrite`, 100k
iterator steps for `ScanAll`, ...), so the standard `ns/op` is **per iteration, not per key**.
Every benchmark therefore also reports:

```
ns/op-key   elapsed / (b.N * opsPerIter)   <- compare the lanes on this
B/s         via b.SetBytes, where values actually move
```

## Ops per iteration

The `ns/op-key` divisor, which must stay equal to the Rust baseline's `Throughput::Elements`:

| workload | ops/iter | workload | ops/iter |
|---|---|---|---|
| SeqWrite | 10000 | ScanAll | 100000 |
| RandWrite | 10000 | ScanReverse | 100000 |
| GetHit | 1000 | ScanPrefix | 1000 |
| GetMiss | 1000 | TxnWrite | 100 |
| Has | 1000 | TxnReadWrite | 20 |
| BatchWrite | 1000 | ParallelMixed | 4000 |
| TxnContendedHot8 | 800 | TxnContendedHot4096 | 800 |
| ScanAllAppend | 100000 | ScanAllBorrow | 100000 |

## Contended transactions

Every other workload here is uncontended: transactions run one at a time, so no commit can
conflict, and the cost of a conflict is never measured. `TxnContended` is the one that does.
It has **no counterpart in the Rust baseline** and is outside the fidelity contract below.

Shape, identical in every lane:

- 4 goroutines (`parallelThreads`, so it is comparable with `ParallelMixed`) commit
  concurrently against **one shared store**, 25 committed transactions each per iteration.
- Each transaction is a read-modify-write over a **hot key range**: 4 `Get`s then 4 `Set`s on
  keys drawn from `key(0)..key(hotN)`. The reads are what make a conflict possible under
  snapshot isolation; the writes are what make other transactions' reads conflict.
- On `corekv.ErrTxnConflict` the whole transaction is replayed from a **fresh** `NewTxn` -
  what a real caller must do. Retries are capped at 100 per logical transaction and hitting
  the cap fails the benchmark, so a livelock shows up as a failure and not as a hang.
- Every transaction eventually commits, and the count of commits is asserted, so the measured
  unit is a **successful** commit. `opsPerIter` counts only committed operations: the time
  spent on attempts that conflicted and were discarded is charged to the survivors, which is
  what the caller actually pays.
- Key *selection* is seeded (its own `newRng(Seed)` walk, so the fidelity-pinned sequences are
  untouched) and therefore identical in every lane and at both value sizes. Which transactions
  actually race is up to the scheduler and is not deterministic.

The contention knob is the hot-range size, and each level is its own top-level benchmark:

| benchmark | hot range | expectation |
|---|---|---|
| `TxnContendedHot8` | 8 keys | heavily contended, conflicts are the norm |
| `TxnContendedHot4096` | 4096 keys | mildly contended, conflicts are rare |

Reported metrics, on top of the usual `ns/op-key`:

```
conflicts/attempt   retries / total commit attempts   <- the conflict rate; the point of this workload
retries/txn         retries / successful commits      <- the same information per committed txn
```

The `memory` lane runs this workload like any other and is **not** skipped: memory is MVCC and
returns `ErrTxnConflict` from `Commit` too, so its number is meaningful and it stays the sanity
control. A lane whose store could not conflict at all would report an honest `0` with the retry
loop never firing - but there is no such lane in this suite, and a store without transactions
could not be a `corekv.TxnStore` in the first place.

## Fidelity with the Rust baseline

`workload.go` mirrors `rust-baseline/benches/workloads.rs` bit-for-bit; changing one without the
other invalidates the comparison. Shared by construction:

- xorshift64* seeded 42, with the multiply as an output scramble that does not feed back into the
  state, and a **descending** Fisher-Yates shuffle using plain `% (i+1)` (the modulo bias is
  identical on both sides and cancels - do not "fix" it).
- Keys `key:%012d` (16 B); values are 0xAB repeated; `GetMiss` asks for `key:{1000000 + order[i]}`.
- `ScanPrefix` prefix `key:000000099`, exactly 1000 of the 100k keys, count asserted.
- Point reads walk the permutation with a wrapping cursor, so the whole key set is touched.
- `ParallelMixed` is 4 goroutines x 1000 ops, put when `n%10 == 9`, goroutine `t` starting at
  `t*len(order)/4`. It deliberately does **not** use `b.RunParallel`: that fixes the goroutine count
  at `p*GOMAXPROCS` and splits `b.N` across it, which cannot express "4 threads of exactly 1000 ops"
  on an 8-CPU machine. Four explicit goroutines per iteration match the Rust `thread::scope` shape.
- `BatchWrite` is a 1000-key transaction, not a native batch primitive - corekv's `TxnStore` has no
  batch API, so a native `WriteBatch` number would have no Go counterpart.
- The transactional workloads (`TxnWrite`, `TxnReadWrite`, `BatchWrite`) run against a store
  prefilled with 1000 keys, as the Rust lane's transactional fixture is.

`fidelity_test.go` pins this: it asserts the permutation against golden values produced by
compiling the Rust `Rng`/`shuffled()` verbatim, and asserts the key format and the prefix's match
count. Run it with plain `go test ./bench`.

Known, deliberate divergences:

- The Rust lane shares one prefilled database between the point reads, the scans **and**
  `ParallelMixed`; here `ParallelMixed` gets its own prefilled store, because it writes and the
  harness never shares a store between a read and a write workload.
- Rust shares one transactional database across the three txn workloads; each Go benchmark gets its
  own, prefilled identically. All three write the same keys with the same value, so the states match.
- Go re-invokes a benchmark body with a growing `b.N`, which restarts the point-read cursor at 0 for
  each attempt. Criterion does not. The sequence of keys is a prefix of the same permutation either
  way.

## Invariants the harness enforces

- Prefill and store construction happen before `b.ResetTimer()`, never inside the timed region.
- Read-only workloads (`GetHit GetMiss Has ScanAll ScanAllAppend ScanAllBorrow ScanReverse
  ScanPrefix`) share one prefilled store per (lane, value size); a store is never shared between a
  read and a write workload.
- Seed 42 for every random ordering, so all lanes touch keys in the same sequence.
- Every error fails the benchmark. `GetMiss` asserts `corekv.ErrNotFound`; the scans assert the
  exact item count, so an empty iterator cannot masquerade as a fast one.
- Errors from a worker goroutine are collected and raised on the benchmark's own goroutine.
  `b.Fatal` off the benchmark goroutine only `runtime.Goexit`s that goroutine, which would
  leave the benchmark running with a worker silently missing.
- badger runs **on disk** in a temp dir (regolith is a disk engine; in-memory badger would not be
  a fair comparison) with `badger.DefaultOptions`. No engine's knobs are tuned, in either lane.

## Adding the regolith lane

1. Build the Rust staticlib: `cd ../regolith && make ffi`.
2. Add to `go.mod`:
   ```
   require github.com/sourcenetwork/corekv/regolith v0.0.0
   replace github.com/sourcenetwork/corekv/regolith v0.0.0 => ../regolith
   ```
3. Fill in `stores_regolith.go` (see `TODO(regolith-lane)` there): uncomment the import and the
   store construction, and set `regolithNoop` to a cgo wrapper whose body is nothing but
   `C.regolith_noop()`.
4. `./run.sh -tags regolith`.

Nothing else in the harness needs to change — the workloads are store-agnostic.
