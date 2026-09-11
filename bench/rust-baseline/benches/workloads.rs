//! Native Rust baseline for the corekv/regolith FFI-overhead benchmark.
//!
//! These are the same twelve workloads the Go lanes (`go-ffi` and `badger`) run,
//! executed against regolith directly with no FFI, no cgo and no marshalling.
//! The numbers here are the floor: `go-ffi - rust` is the cost of the boundary.
//!
//! Fidelity rules (do not change one side without the other):
//!   * keys are `format!("key:{i:012}")` — 16 bytes, identical to Go's
//!     `fmt.Sprintf("key:%012d", i)`;
//!   * random orders come from the `Rng` below, seeded 42, shuffled with the
//!     descending Fisher-Yates in `shuffled()`. The Go lane reproduces this
//!     algorithm bit-for-bit;
//!   * the shipping configuration — serializable isolation and
//!     `transaction_keys_inline = 4096`, default `DurabilityMode::Eventual`;
//!   * on-disk in a `tempfile::TempDir`, never `MemEnv`;
//!   * prefill and database open happen outside every measured closure.
//!
//! Every benchmark declares `Throughput::Elements(n)` where `n` is the number of
//! store operations in one criterion iteration, so criterion's reported
//! iteration time divided by `n` is the per-operation cost. See README.md.

use std::cell::Cell;
use std::hint::black_box;
use std::thread;

use criterion::{criterion_group, criterion_main, Criterion, Throughput};
use regolith::{
    Db, IsolationLevel, OptimisticTransactionDb, Options, WriteBatch,
};
use tempfile::TempDir;

// ---------------------------------------------------------------------------
// Workload spec constants — mirrored in the Go lane.
// ---------------------------------------------------------------------------

/// Keys written per `SeqWrite` / `RandWrite` iteration.
const WRITE_N: u64 = 10_000;
/// Keys present in the read-only fixture.
const PREFILL_N: u64 = 100_000;
/// Point operations per iteration for Get/Has workloads.
const POINT_OPS: u64 = 1_000;
/// `ScanPrefix` walks `key:000000099xxx` — exactly 1000 of the 100k keys.
const SCAN_PREFIX: &[u8] = b"key:000000099";
const SCAN_PREFIX_N: u64 = 1_000;
const TXN_WRITE_N: u64 = 100;
const TXN_RW_N: u64 = 10;
const BATCH_WRITE_N: u64 = 1_000;
const PARALLEL_THREADS: u64 = 4;
const PARALLEL_OPS_PER_THREAD: u64 = 1_000;

/// The two value sizes from the spec table, reported separately.
const SIZES: [(&str, usize); 2] = [("64B", 64), ("4KiB", 4096)];

// ---------------------------------------------------------------------------
// Deterministic PRNG — THE GO LANE MUST REPRODUCE THIS EXACTLY.
// ---------------------------------------------------------------------------

/// xorshift64* (Marsaglia/Vigna), seeded with 42.
///
/// State is a single `u64`, initialised to the seed (42, which is non-zero, so
/// the generator never degenerates). Each `next_u64` call does, in order, with
/// wrapping arithmetic and logical (unsigned) shifts:
///
/// ```text
///   x ^= x >> 12
///   x ^= x << 25
///   x ^= x >> 27
///   return x * 0x2545F4914F6CDD1D
/// ```
///
/// The multiply is an output scramble only; it does not feed back into state.
///
/// Go equivalent:
///
/// ```go
/// type rng struct{ x uint64 }
///
/// func newRng(seed uint64) *rng { return &rng{x: seed} }
///
/// func (r *rng) next() uint64 {
///     r.x ^= r.x >> 12
///     r.x ^= r.x << 25
///     r.x ^= r.x >> 27
///     return r.x * 0x2545F4914F6CDD1D
/// }
/// ```
struct Rng {
    x: u64,
}

impl Rng {
    fn new(seed: u64) -> Self {
        assert!(seed != 0, "xorshift64* state must be non-zero");
        Self { x: seed }
    }

    fn next_u64(&mut self) -> u64 {
        self.x ^= self.x >> 12;
        self.x ^= self.x << 25;
        self.x ^= self.x >> 27;
        self.x.wrapping_mul(0x2545_F491_4F6C_DD1D)
    }
}

/// `[0, n)` shuffled with a descending Fisher-Yates driven by `Rng::new(42)`.
///
/// The index is `next_u64() % (i + 1)` — modulo, not rejection sampling, so the
/// bias is identical on both sides of the comparison. Go equivalent:
///
/// ```go
/// func shuffled(n uint64) []uint64 {
///     v := make([]uint64, n)
///     for i := range v {
///         v[i] = uint64(i)
///     }
///     r := newRng(42)
///     for i := n - 1; i > 0; i-- {
///         j := r.next() % (i + 1)
///         v[i], v[j] = v[j], v[i]
///     }
///     return v
/// }
/// ```
fn shuffled(n: u64) -> Vec<u64> {
    let mut v: Vec<u64> = (0..n).collect();
    let mut rng = Rng::new(42);
    let mut i = n - 1;
    while i > 0 {
        let j = (rng.next_u64() % (i + 1)) as usize;
        v.swap(i as usize, j);
        i -= 1;
    }
    v
}

// ---------------------------------------------------------------------------
// Fixtures
// ---------------------------------------------------------------------------

fn key(i: u64) -> Vec<u8> {
    format!("key:{i:012}").into_bytes()
}

/// The configuration the Go lane ships, so the two are comparable.
///
/// `transaction_keys_inline` above the largest transaction in the suite: past
/// this many keys the transaction write buffer indexes itself and clones the
/// key on every later insert, which a transaction that only writes never reads
/// back. Everything else is `Options::default()`, including
/// `DurabilityMode::Eventual`.
///
/// This matters for honesty rather than for speed. Running this lane at the
/// engine defaults while the Go lane is tuned made go-regolith appear to beat
/// regolith-called-from-Rust on the transactional workloads, which is not a
/// thing that can happen and made the boundary cost unreadable on those rows.
fn shipping_options() -> Options {
    Options {
        transaction_keys_inline: 4096,
        ..Options::default()
    }
}

/// On disk, in the shipping configuration. The `TempDir` is returned so the
/// caller keeps the directory alive for the database's whole life.
fn open_db(tag: &str) -> (TempDir, Db) {
    let dir = TempDir::new().expect("create tempdir");
    let db = Db::open(dir.path(), shipping_options())
        .unwrap_or_else(|e| panic!("open db for {tag}: {e}"));
    (dir, db)
}

fn open_txn_db(tag: &str) -> (TempDir, OptimisticTransactionDb) {
    let dir = TempDir::new().expect("create tempdir");
    let db = OptimisticTransactionDb::open(dir.path(), shipping_options())
        .unwrap_or_else(|e| panic!("open txn db for {tag}: {e}"));
    (dir, db)
}

/// Write `key:0 .. key:n` with `value`, batched for speed. Never measured.
fn prefill(db: &Db, n: u64, value: &[u8]) {
    let mut batch = WriteBatch::new();
    for i in 0..n {
        batch.put(&key(i), value);
        if batch.len() >= 1_000 {
            db.write(std::mem::replace(&mut batch, WriteBatch::new()))
                .expect("prefill batch write");
        }
    }
    if !batch.is_empty() {
        db.write(batch).expect("prefill tail write");
    }
    db.flush().expect("prefill flush");
}

/// A cursor that walks a fixed permutation, wrapping — the Rust analogue of the
/// Go lane's `order[i%len(order)]` inside a `b.N` loop. Every key in the
/// permutation is eventually touched rather than just the first slice of it.
struct Cursor {
    order: Vec<u64>,
    at: Cell<usize>,
}

impl Cursor {
    fn new(order: Vec<u64>) -> Self {
        Self { order, at: Cell::new(0) }
    }

    fn next(&self) -> u64 {
        let at = self.at.get();
        self.at.set((at + 1) % self.order.len());
        self.order[at]
    }
}

// ---------------------------------------------------------------------------
// Benchmarks
// ---------------------------------------------------------------------------

/// 1. `SeqWrite` — 10k sequential `put`, no transaction.
/// 2. `RandWrite` — the same 10k keys in shuffled order.
///
/// One database per workload, opened outside the measurement. Successive
/// iterations overwrite the same key set, exactly as the Go lane's `b.N` loop
/// does; the engine therefore sees overwrites after the first iteration.
fn bench_writes(c: &mut Criterion, label: &str, value: &[u8]) {
    let order = shuffled(WRITE_N);

    for (name, keys) in [
        ("SeqWrite", (0..WRITE_N).collect::<Vec<_>>()),
        ("RandWrite", order),
    ] {
        let keys: Vec<Vec<u8>> = keys.into_iter().map(key).collect();
        let (_dir, db) = open_db(name);

        let mut g = c.benchmark_group(format!("{name}/{label}"));
        g.throughput(Throughput::Elements(WRITE_N));
        g.bench_function("put", |b| {
            b.iter(|| {
                for k in &keys {
                    db.put(k, value).expect("put");
                }
            })
        });
        g.finish();
    }
}

/// 3. `GetHit`, 4. `GetMiss`, 5. `Has` — 100k prefill, random access order.
///
/// `GetMiss` asks for `key:1000000xxxxxx`, which the prefill never wrote, and
/// asserts the answer is `None` so a silent hit cannot masquerade as a miss.
fn bench_points(c: &mut Criterion, label: &str, db: &Db) {
    let hit = Cursor::new(shuffled(PREFILL_N));
    let miss = Cursor::new(shuffled(PREFILL_N));
    let has = Cursor::new(shuffled(PREFILL_N));

    let mut g = c.benchmark_group(format!("GetHit/{label}"));
    g.throughput(Throughput::Elements(POINT_OPS));
    g.bench_function("get", |b| {
        b.iter(|| {
            for _ in 0..POINT_OPS {
                let k = key(hit.next());
                let v = db.get(&k).expect("get hit");
                assert!(v.is_some(), "GetHit missed a prefilled key");
                black_box(v);
            }
        })
    });
    g.finish();

    let mut g = c.benchmark_group(format!("GetMiss/{label}"));
    g.throughput(Throughput::Elements(POINT_OPS));
    g.bench_function("get", |b| {
        b.iter(|| {
            for _ in 0..POINT_OPS {
                let k = key(1_000_000 + miss.next());
                let v = db.get(&k).expect("get miss");
                assert!(v.is_none(), "GetMiss found a key that was never written");
                black_box(v);
            }
        })
    });
    g.finish();

    let mut g = c.benchmark_group(format!("Has/{label}"));
    g.throughput(Throughput::Elements(POINT_OPS));
    g.bench_function("has", |b| {
        b.iter(|| {
            for _ in 0..POINT_OPS {
                let k = key(has.next());
                let present = db.has(&k).expect("has");
                assert!(present, "Has missed a prefilled key");
                black_box(present);
            }
        })
    });
    g.finish();
}

/// 6. `ScanAll`, 7. `ScanReverse`, 8. `ScanPrefix` — over the 100k prefill.
///
/// `Snapshot::owned_iter()` is the cursor the FFI layer will expose, so the
/// baseline uses it too rather than the borrowing `Snapshot::iter()`. Values are
/// summed so the engine cannot elide the value read; reverse iteration is
/// `seek_to_last` + `prev`, per the cursor API.
fn bench_scans(c: &mut Criterion, label: &str, db: &Db) {
    let mut g = c.benchmark_group(format!("ScanAll/{label}"));
    g.throughput(Throughput::Elements(PREFILL_N));
    g.bench_function("iter", |b| {
        b.iter(|| {
            let snap = db.snapshot();
            let mut it = snap.owned_iter();
            it.seek_to_first();
            let mut n = 0u64;
            let mut bytes = 0usize;
            while it.valid() {
                bytes += it.value().expect("value at valid cursor").len();
                n += 1;
                it.next();
            }
            it.status().expect("scan status");
            assert_eq!(n, PREFILL_N, "ScanAll saw the wrong number of keys");
            black_box(bytes);
        })
    });
    g.finish();

    let mut g = c.benchmark_group(format!("ScanReverse/{label}"));
    g.throughput(Throughput::Elements(PREFILL_N));
    g.bench_function("iter", |b| {
        b.iter(|| {
            let snap = db.snapshot();
            let mut it = snap.owned_iter();
            it.seek_to_last();
            let mut n = 0u64;
            let mut bytes = 0usize;
            while it.valid() {
                bytes += it.value().expect("value at valid cursor").len();
                n += 1;
                it.prev();
            }
            it.status().expect("scan status");
            assert_eq!(n, PREFILL_N, "ScanReverse saw the wrong number of keys");
            black_box(bytes);
        })
    });
    g.finish();

    let mut g = c.benchmark_group(format!("ScanPrefix/{label}"));
    g.throughput(Throughput::Elements(SCAN_PREFIX_N));
    g.bench_function("iter", |b| {
        b.iter(|| {
            let snap = db.snapshot();
            let mut it = snap.owned_iter();
            it.seek_prefix(SCAN_PREFIX);
            let mut n = 0u64;
            let mut bytes = 0usize;
            while it.valid() {
                bytes += it.value().expect("value at valid cursor").len();
                n += 1;
                it.next();
            }
            it.status().expect("scan status");
            assert_eq!(n, SCAN_PREFIX_N, "ScanPrefix saw the wrong number of keys");
            black_box(bytes);
        })
    });
    g.finish();
}

/// 9. `TxnWrite`, 10. `TxnReadWrite`, 11. `BatchWrite`.
///
/// `OptimisticTransactionDb` + `IsolationLevel::Serializable`, matching the
/// configuration the Go store ships. Serializable costs nothing outside
/// contention - measured within noise on every uncontended transactional
/// workload - and it is what makes the badger comparison honest, since
/// snapshot isolation admits write skew that badger's SSI rejects. Transactions run one at a time, so no commit
/// can conflict; a conflict here would be a bug and is therefore fatal.
///
/// Note on `BatchWrite`: the spec table defines it as a 1000-key transaction,
/// not as `Db::write(WriteBatch)`. corekv's `TxnStore` has no batch primitive,
/// so a native `WriteBatch` measurement would have nothing to compare against.
/// Kept as a transaction for that reason.
fn bench_txns(c: &mut Criterion, label: &str, value: &[u8]) {
    let (_dir, db) = open_txn_db("txn");
    // Enough keys present that TxnReadWrite's gets are hits.
    prefill(db.db(), BATCH_WRITE_N, value);

    let begin = || db.begin_transaction_with(IsolationLevel::Serializable);

    let mut g = c.benchmark_group(format!("TxnWrite/{label}"));
    g.throughput(Throughput::Elements(TXN_WRITE_N));
    g.bench_function("txn", |b| {
        b.iter(|| {
            let txn = begin();
            for i in 0..TXN_WRITE_N {
                txn.put(&key(i), value).expect("txn put");
            }
            txn.commit().expect("txn commit");
        })
    });
    g.finish();

    // 10 gets + 10 puts = 20 store operations per iteration.
    let mut g = c.benchmark_group(format!("TxnReadWrite/{label}"));
    g.throughput(Throughput::Elements(TXN_RW_N * 2));
    g.bench_function("txn", |b| {
        b.iter(|| {
            let txn = begin();
            for i in 0..TXN_RW_N {
                let v = txn.get(&key(i)).expect("txn get");
                assert!(v.is_some(), "TxnReadWrite missed a prefilled key");
                black_box(v);
            }
            for i in 0..TXN_RW_N {
                txn.put(&key(i), value).expect("txn put");
            }
            txn.commit().expect("txn commit");
        })
    });
    g.finish();

    let mut g = c.benchmark_group(format!("BatchWrite/{label}"));
    g.throughput(Throughput::Elements(BATCH_WRITE_N));
    g.bench_function("txn", |b| {
        b.iter(|| {
            let txn = begin();
            for i in 0..BATCH_WRITE_N {
                txn.put(&key(i), value).expect("txn put");
            }
            txn.commit().expect("txn commit");
        })
    });
    g.finish();
}

/// 12. `ParallelMixed` — 4 threads, 90% `get` / 10% `put` over the 100k prefill.
///
/// The Go lane's `b.RunParallel` body is the same 9-gets-then-1-put cycle. Each
/// thread owns its own cursor into a distinct shuffled permutation (seeded from
/// the shared 42-seeded order, rotated by thread index) so the threads do not
/// all hammer the same key at the same time.
fn bench_parallel_mixed(c: &mut Criterion, label: &str, db: &Db, value: &[u8]) {
    let order = shuffled(PREFILL_N);
    let total = PARALLEL_THREADS * PARALLEL_OPS_PER_THREAD;

    let mut g = c.benchmark_group(format!("ParallelMixed/{label}"));
    g.throughput(Throughput::Elements(total));
    g.bench_function("mixed", |b| {
        b.iter(|| {
            thread::scope(|s| {
                for t in 0..PARALLEL_THREADS {
                    let order = &order;
                    s.spawn(move || {
                        let base = (t as usize) * (order.len() / PARALLEL_THREADS as usize);
                        for n in 0..PARALLEL_OPS_PER_THREAD {
                            let k = key(order[(base + n as usize) % order.len()]);
                            if n % 10 == 9 {
                                db.put(&k, value).expect("parallel put");
                            } else {
                                let v = db.get(&k).expect("parallel get");
                                black_box(v);
                            }
                        }
                    });
                }
            })
        })
    });
    g.finish();
}

fn all(c: &mut Criterion) {
    for (label, vsize) in SIZES {
        let value = vec![0xABu8; vsize];

        bench_writes(c, label, &value);
        bench_txns(c, label, &value);

        // One prefilled fixture shared by every read-shaped workload. Opened and
        // filled here, outside every measured closure.
        let (_dir, db) = open_db("reads");
        prefill(&db, PREFILL_N, &value);
        bench_points(c, label, &db);
        bench_scans(c, label, &db);
        bench_parallel_mixed(c, label, &db, &value);
    }
}

criterion_group!(baseline, all);
criterion_main!(baseline);
