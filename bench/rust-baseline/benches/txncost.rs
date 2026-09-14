//! Where does regolith's transactional write cost go?
//!
//! This file is a diagnosis harness, not part of the thirteen-workload spec in
//! `workloads.rs`, and it is not compared against the Go lanes. Every number is
//! compared against another number measured in this same file, on this machine,
//! in the same run.
//!
//! # Lanes
//!
//! Two environments, because the first exploratory run showed the on-disk
//! numbers carrying a ±25% confidence interval — wide enough to swallow the
//! effect being hunted:
//!
//!   * `disk` — `Options::default()` in a `TempDir`, exactly what `workloads.rs`
//!     uses. The fidelity lane: whatever it says is what a real deployment
//!     pays, noise and all.
//!   * `mem` — the same options with `env: MemEnv`. Removes the filesystem, so
//!     the WAL append and the SSTable flush become memcpys. The attribution
//!     lane: a difference that survives here is CPU-side work in regolith, not
//!     I/O.
//!
//! And a WAL axis on the non-transactional paths (`WriteOptions::disable_wal`),
//! to size the log append. Note that there is deliberately no transactional
//! counterpart: `commit_optimistic` hardcodes `disable_wal: false`
//! (regolith-0.1.4 `src/engine/commit/mod.rs:255`), so a transaction cannot opt
//! out of the WAL even when the caller would like to.
//!
//! # Conventions
//!
//!   * keys precomputed outside every measured closure — `format!` is not part
//!     of what is being sized;
//!   * `Throughput::Elements(n)` with `n` = keys touched, so criterion's
//!     per-iteration time divided by `n` is ns per key-operation, the unit the
//!     investigation is stated in;
//!   * phase splits use `iter_batched` with `BatchSize::PerIteration`. Criterion
//!     does not time `setup`, so `commit-phase` times `commit()` alone with the
//!     puts already buffered. `PerIteration` rather than `SmallInput` matters:
//!     `SmallInput` would build a whole batch of transactions before timing any
//!     of them, leaving thousands of registered snapshots alive at once and
//!     pinning the read horizon, which changes the thing being measured.

use std::hint::black_box;
use std::sync::Arc;
use std::time::Duration;

use criterion::measurement::WallTime;
use criterion::{
    criterion_group, criterion_main, BatchSize, BenchmarkGroup, BenchmarkId, Criterion, Throughput,
};
use regolith::env::MemEnv;
use regolith::{
    Db, IsolationLevel, OptimisticTransactionDb, Options, TransactionDb, WriteBatch, WriteOptions,
};
use tempfile::TempDir;

/// Value sizes: the per-byte / per-op discriminator.
const SIZES: [(&str, usize); 2] = [("64B", 64), ("4KiB", 4096)];

/// Transaction sizes for the amortisation curve.
const TXN_SIZES: [u64; 4] = [1, 10, 100, 1000];

/// The shape the headline finding is stated at.
const N: u64 = 100;

#[derive(Copy, Clone, PartialEq)]
enum Lane {
    Disk,
    Mem,
}

impl Lane {
    fn tag(self) -> &'static str {
        match self {
            Lane::Disk => "disk",
            Lane::Mem => "mem",
        }
    }

    /// `Options::default()` for `Disk`; the same with a fresh `MemEnv` for
    /// `Mem`. Nothing else is tuned in either lane.
    fn options(self) -> Options {
        match self {
            Lane::Disk => Options::default(),
            // `max_background_compactions: 0` is not a tuning choice: `MemEnv`
            // cannot spawn threads, and `Db::open` rejects any other value for
            // it. `write_buffer_size` is raised to 1 GiB below so that this lane
            // never flushes mid-measurement, which would put a memcpy of the
            // whole memtable inside one unlucky sample. Everything else is
            // `Options::default()`.
            Lane::Mem => Options {
                env: Arc::new(MemEnv::default()),
                max_background_compactions: 0,
                write_buffer_size: 1 << 30,
                ..Options::default()
            },
        }
    }
}

/// Sampling, set in code rather than on the command line so a reported run is
/// reproducible with a bare `cargo bench --bench txncost`.
///
/// 100 samples over a short window rather than criterion's default 3 s, because
/// the bytes written during a measurement do not go away: neither lane rotates
/// its WAL until the memtable fills, so a long window grows one append-only file
/// to hundreds of megabytes and the reallocation that copies it lands inside
/// some unlucky sample. A first full run at 3 s produced ±85% intervals on the
/// write-path rows and two measurements of the identical `whole` benchmark
/// 32% apart; the same benchmarks at this setting sit inside ±5%. Samples, not
/// seconds, are what narrows the interval here.
///
/// Heavy groups — 4 KiB values, or 1000 keys per iteration — are capped harder
/// still, because in the `mem` lane every byte stays resident in the `MemEnv`
/// filesystem for the life of the group.
fn tune(g: &mut BenchmarkGroup<'_, WallTime>, bytes_per_iter: usize) {
    if bytes_per_iter > 64 * 1024 {
        g.sample_size(50)
            .warm_up_time(Duration::from_millis(200))
            .measurement_time(Duration::from_millis(500));
    } else {
        g.sample_size(100)
            .warm_up_time(Duration::from_millis(300))
            .measurement_time(Duration::from_millis(800));
    }
}

/// Bytes one iteration stages: `n` keys of `value.len()` plus ~20 bytes of key.
fn per_iter_bytes(n: u64, value: &[u8]) -> usize {
    n as usize * (value.len() + 20)
}

fn key(i: u64) -> Vec<u8> {
    format!("key:{i:012}").into_bytes()
}

fn keys(n: u64) -> Vec<Vec<u8>> {
    (0..n).map(key).collect()
}

fn open_db(lane: Lane) -> (TempDir, Db) {
    let dir = TempDir::new().expect("tempdir");
    let db = Db::open(dir.path(), lane.options()).expect("open db");
    (dir, db)
}

fn open_opt(lane: Lane) -> (TempDir, OptimisticTransactionDb) {
    let dir = TempDir::new().expect("tempdir");
    let db = OptimisticTransactionDb::open(dir.path(), lane.options()).expect("open opt db");
    (dir, db)
}

fn open_pes(lane: Lane) -> (TempDir, TransactionDb) {
    let dir = TempDir::new().expect("tempdir");
    let db = TransactionDb::open(dir.path(), lane.options()).expect("open pes db");
    (dir, db)
}

// ---------------------------------------------------------------------------
// Non-transactional baselines: the direct put and the atomic batch.
// ---------------------------------------------------------------------------

/// `Db::put` over `n` keys, with and without the WAL.
///
/// `n` puts are `n` engine commits: `put_opt` calls `apply_single_put`, which
/// submits one `WriteRequest::Put` to the commit pipeline
/// (`src/engine/commit/mod.rs:182-200`). That is the asymmetry with a
/// transaction, which submits one request for the whole write set, and it is
/// why this baseline has to be read per key rather than per call.
fn bench_direct_put(c: &mut Criterion, lane: Lane, label: &str, value: &[u8], n: u64) {
    let ks = keys(n);
    let (_dir, db) = open_db(lane);

    let mut g = c.benchmark_group(format!("DirectPut/{}/{label}/{n}", lane.tag()));
    g.throughput(Throughput::Elements(n));
    tune(&mut g, per_iter_bytes(n, value));
    g.bench_function("put", |b| {
        b.iter(|| {
            for k in &ks {
                db.put(k, value).expect("put");
            }
        })
    });
    let no_wal = WriteOptions::disable_wal();
    g.bench_function("put-nowal", |b| {
        b.iter(|| {
            for k in &ks {
                db.put_opt(&no_wal, k, value).expect("put");
            }
        })
    });
    g.finish();
}

/// `Db::write(WriteBatch)` — the non-transactional atomic batch, and the
/// closest non-transactional analogue of a transaction commit: one engine
/// request carrying `n` point writes.
///
/// Three variants:
///   * `build+write` builds the batch inside the measurement, so it pays the
///     same per-key key copy and value copy `Transaction::put` pays;
///   * `write-only` builds it in untimed setup, isolating the engine apply;
///   * `write-only-nowal` drops the log append as well.
fn bench_batch(c: &mut Criterion, lane: Lane, label: &str, value: &[u8], n: u64) {
    let ks = keys(n);
    let (_dir, db) = open_db(lane);
    let build = || {
        let mut batch = WriteBatch::new();
        for k in &ks {
            batch.put(k, value);
        }
        batch
    };

    let mut g = c.benchmark_group(format!("BatchWriteDirect/{}/{label}/{n}", lane.tag()));
    g.throughput(Throughput::Elements(n));
    tune(&mut g, per_iter_bytes(n, value));
    g.bench_function("build+write", |b| {
        b.iter(|| db.write(build()).expect("batch write"))
    });
    g.bench_function("write-only", |b| {
        b.iter_batched(
            build,
            |batch| db.write(batch).expect("batch write"),
            BatchSize::PerIteration,
        )
    });
    let no_wal = WriteOptions::disable_wal();
    g.bench_function("write-only-nowal", |b| {
        b.iter_batched(
            build,
            |batch| db.write_opt(&no_wal, batch).expect("batch write"),
            BatchSize::PerIteration,
        )
    });
    g.finish();
}

// ---------------------------------------------------------------------------
// 1. The split: begin/rollback frame, put phase, commit phase.
// ---------------------------------------------------------------------------

/// One `n`-key optimistic `SnapshotIsolation` transaction at the default
/// `transaction_keys_inline` (32), taken apart.
///
/// This is not the configuration `TxnWrite` in `workloads.rs` runs: that one is
/// `Serializable` at `transaction_keys_inline` 4096. The rows in this group are
/// comparable with each other, never with `workloads.rs`.
///
///   * `whole` — begin + n puts + commit. Reproduces `TxnWrite`.
///   * `put-phase` — begin + n puts + rollback. Buffering only, no engine write.
///   * `begin-rollback` — begin + rollback with zero puts. The frame to subtract
///     from `put-phase` to leave the cost of buffering alone.
///   * `commit-phase` — `commit()` with the n puts done in untimed setup.
///
/// `put-phase + commit-phase` should reconstruct `whole` to within the noise
/// floor (the begin/rollback frame is inside `put-phase` already). If it does
/// not, the split is lying and none of it should be believed.
fn bench_split(c: &mut Criterion, lane: Lane, label: &str, value: &[u8], n: u64) {
    let ks = keys(n);
    let (_dir, db) = open_opt(lane);
    let begin = || db.begin_transaction_with(IsolationLevel::SnapshotIsolation);

    let mut g = c.benchmark_group(format!("TxnSplit/{}/{label}/{n}", lane.tag()));
    g.throughput(Throughput::Elements(n));
    tune(&mut g, per_iter_bytes(n, value));

    g.bench_function("whole", |b| {
        b.iter(|| {
            let txn = begin();
            for k in &ks {
                txn.put(k, value).expect("txn put");
            }
            txn.commit().expect("commit");
        })
    });

    g.bench_function("put-phase", |b| {
        b.iter(|| {
            let txn = begin();
            for k in &ks {
                txn.put(k, value).expect("txn put");
            }
            txn.rollback();
        })
    });

    g.bench_function("begin-rollback", |b| {
        b.iter(|| {
            let txn = begin();
            black_box(&txn);
            txn.rollback();
        })
    });

    g.bench_function("commit-phase", |b| {
        b.iter_batched(
            || {
                let txn = begin();
                for k in &ks {
                    txn.put(k, value).expect("txn put");
                }
                txn
            },
            |txn| txn.commit().expect("commit"),
            BatchSize::PerIteration,
        )
    });

    g.finish();
}

// ---------------------------------------------------------------------------
// 2. Isolation levels.
// ---------------------------------------------------------------------------

/// The same write-only transaction at all three isolation levels.
///
/// The prediction read off `Transaction::validation_set`
/// (`src/transaction.rs:916-985`) is that the level makes no difference to a
/// write-only *optimistic* transaction: the `if optimistic { for key in
/// writes.keys() { checks.entry(..).or_insert(..) } }` block at
/// `src/transaction.rs:964-976` adds every written key to the validation set
/// unconditionally, and the level only decides which *read* keys join it. A flat
/// result confirms that reading; any spread refutes it.
fn bench_isolation(c: &mut Criterion, lane: Lane, label: &str, value: &[u8]) {
    let ks = keys(N);
    let (_dir, db) = open_opt(lane);

    let mut g = c.benchmark_group(format!("TxnIsolation/{}/{label}", lane.tag()));
    g.throughput(Throughput::Elements(N));
    tune(&mut g, per_iter_bytes(N, value));
    for (name, level) in [
        ("ReadCommitted", IsolationLevel::ReadCommitted),
        ("SnapshotIsolation", IsolationLevel::SnapshotIsolation),
        ("Serializable", IsolationLevel::Serializable),
    ] {
        g.bench_function(BenchmarkId::new("whole", name), |b| {
            b.iter(|| {
                let txn = db.begin_transaction_with(level);
                for k in &ks {
                    txn.put(k, value).expect("txn put");
                }
                txn.commit().expect("commit");
            })
        });
        g.bench_function(BenchmarkId::new("commit-phase", name), |b| {
            b.iter_batched(
                || {
                    let txn = db.begin_transaction_with(level);
                    for k in &ks {
                        txn.put(k, value).expect("txn put");
                    }
                    txn
                },
                |txn| txn.commit().expect("commit"),
                BatchSize::PerIteration,
            )
        });
    }
    g.finish();
}

// ---------------------------------------------------------------------------
// 3. Pessimistic vs optimistic.
// ---------------------------------------------------------------------------

/// `TransactionDb` (pessimistic) against `OptimisticTransactionDb` for the same
/// `N` blind puts.
///
/// This is the cleanest probe of the commit-time validation cost available from
/// outside the engine, and it works because of an asymmetry in `validation_set`:
/// the per-write-key loop is inside `if optimistic`
/// (`src/transaction.rs:964-976`), so in pessimistic mode a key that was written
/// but never read is left out of the validation set entirely. A pessimistic
/// blind-write commit therefore does *zero* `latest_version_seq_in_view`
/// lookups where the optimistic one does `n`. What it pays instead is `n`
/// lock-manager acquisitions during the put phase. Splitting both sides lets the
/// two costs be read off separately instead of netted against each other.
fn bench_concurrency_mode(c: &mut Criterion, lane: Lane, label: &str, value: &[u8]) {
    let ks = keys(N);
    let (_opt_dir, opt) = open_opt(lane);
    let (_pes_dir, pes) = open_pes(lane);

    let mut g = c.benchmark_group(format!("TxnMode/{}/{label}", lane.tag()));
    g.throughput(Throughput::Elements(N));
    tune(&mut g, per_iter_bytes(N, value));

    macro_rules! lanes {
        ($tag:literal, $db:expr) => {
            g.bench_function(concat!($tag, "/whole"), |b| {
                b.iter(|| {
                    let txn = $db.begin_transaction_with(IsolationLevel::SnapshotIsolation);
                    for k in &ks {
                        txn.put(k, value).expect("txn put");
                    }
                    txn.commit().expect("commit");
                })
            });
            g.bench_function(concat!($tag, "/put-phase"), |b| {
                b.iter(|| {
                    let txn = $db.begin_transaction_with(IsolationLevel::SnapshotIsolation);
                    for k in &ks {
                        txn.put(k, value).expect("txn put");
                    }
                    txn.rollback();
                })
            });
            g.bench_function(concat!($tag, "/commit-phase"), |b| {
                b.iter_batched(
                    || {
                        let txn = $db.begin_transaction_with(IsolationLevel::SnapshotIsolation);
                        for k in &ks {
                            txn.put(k, value).expect("txn put");
                        }
                        txn
                    },
                    |txn| txn.commit().expect("commit"),
                    BatchSize::PerIteration,
                )
            });
        };
    }

    lanes!("optimistic", opt);
    lanes!("pessimistic", pes);
    g.finish();
}

// ---------------------------------------------------------------------------
// 4. Transaction-size sensitivity: fixed per-commit vs per-key cost.
// ---------------------------------------------------------------------------

/// 1, 10, 100 and 1000 puts per commit, reported per key-operation. A cost that
/// is fixed per commit falls as `1/n` across the sweep; a per-key cost is flat.
/// `Db::put` over the same counts is the per-key control: it has no commit to
/// amortise, so it should be flat.
fn bench_sizes(c: &mut Criterion, lane: Lane, label: &str, value: &[u8]) {
    let (_dir, db) = open_opt(lane);
    let (_ddir, plain) = open_db(lane);

    let mut g = c.benchmark_group(format!("TxnSize/{}/{label}", lane.tag()));
    // Tuned for the largest shape in the sweep, so the whole group shares one
    // sampling configuration and the four points stay comparable.
    tune(&mut g, per_iter_bytes(*TXN_SIZES.last().expect("non-empty"), value));
    for n in TXN_SIZES {
        let ks = keys(n);
        g.throughput(Throughput::Elements(n));
        g.bench_with_input(BenchmarkId::new("txn", n), &n, |b, _| {
            b.iter(|| {
                let txn = db.begin_transaction_with(IsolationLevel::SnapshotIsolation);
                for k in &ks {
                    txn.put(k, value).expect("txn put");
                }
                txn.commit().expect("commit");
            })
        });
        g.bench_with_input(BenchmarkId::new("commit-phase", n), &n, |b, _| {
            b.iter_batched(
                || {
                    let txn = db.begin_transaction_with(IsolationLevel::SnapshotIsolation);
                    for k in &ks {
                        txn.put(k, value).expect("txn put");
                    }
                    txn
                },
                |txn| txn.commit().expect("commit"),
                BatchSize::PerIteration,
            )
        });
        g.bench_with_input(BenchmarkId::new("direct-put", n), &n, |b, _| {
            b.iter(|| {
                for k in &ks {
                    plain.put(k, value).expect("put");
                }
            })
        });
    }
    g.finish();
}

// ---------------------------------------------------------------------------
// 5. Validation against a database the keys are *not* in the memtable of.
// ---------------------------------------------------------------------------

/// The same 100-key transaction, but over a database large enough that the keys
/// it writes have been flushed out of the active memtable.
///
/// Why this lane exists: every other benchmark here rewrites the same 100 keys
/// every iteration, so they are always in the active memtable and
/// `latest_version_seq_in_view` (`src/engine/mod.rs:2181-2250`) answers from its
/// very first probe — `view.active.get(&lk)` at `src/engine/mod.rs:2195`. That is
/// the cheapest validation can ever be. A real workload writing keys it has not
/// just written makes that probe miss the active memtable, miss every frozen
/// one, and descend the levels. This lane measures that, and the
/// `pessimistic/commit-phase` row beside it — which validates nothing at all for
/// blind writes — is the control that isolates it.
///
/// `PREFILL` keys are written and flushed outside every measurement; the
/// transaction then overwrites a 100-key window far from the keys any other
/// lane touches.
fn bench_cold_validation(c: &mut Criterion, lane: Lane, label: &str, value: &[u8]) {
    const PREFILL: u64 = 200_000;
    // A window inside the prefill, so the writes are overwrites of keys that
    // exist but are not in the active memtable.
    let ks: Vec<Vec<u8>> = (120_000..120_000 + N).map(key).collect();

    let fill = |db: &Db| {
        let mut batch = WriteBatch::new();
        for i in 0..PREFILL {
            batch.put(&key(i), value);
            if batch.len() >= 1_000 {
                db.write(std::mem::replace(&mut batch, WriteBatch::new()))
                    .expect("prefill");
            }
        }
        if !batch.is_empty() {
            db.write(batch).expect("prefill tail");
        }
        // The flush is the point: it moves the prefill out of the memtable and
        // into SSTables, so the validation probe has to descend.
        db.flush().expect("prefill flush");
    };

    let (_odir, opt) = open_opt(lane);
    fill(opt.db());
    let (_pdir, pes) = open_pes(lane);
    fill(pes.db());
    let (_bdir, plain) = open_db(lane);
    fill(&plain);

    let mut g = c.benchmark_group(format!("TxnCold/{}/{label}", lane.tag()));
    g.throughput(Throughput::Elements(N));
    tune(&mut g, per_iter_bytes(N, value));

    g.bench_function("optimistic/commit-phase", |b| {
        b.iter_batched(
            || {
                let txn = opt.begin_transaction_with(IsolationLevel::SnapshotIsolation);
                for k in &ks {
                    txn.put(k, value).expect("txn put");
                }
                txn
            },
            |txn| txn.commit().expect("commit"),
            BatchSize::PerIteration,
        )
    });
    g.bench_function("pessimistic/commit-phase", |b| {
        b.iter_batched(
            || {
                let txn = pes.begin_transaction_with(IsolationLevel::SnapshotIsolation);
                for k in &ks {
                    txn.put(k, value).expect("txn put");
                }
                txn
            },
            |txn| txn.commit().expect("commit"),
            BatchSize::PerIteration,
        )
    });
    g.bench_function("batch/write-only", |b| {
        b.iter_batched(
            || {
                let mut batch = WriteBatch::new();
                for k in &ks {
                    batch.put(k, value);
                }
                batch
            },
            |batch| plain.write(batch).expect("batch write"),
            BatchSize::PerIteration,
        )
    });
    g.finish();
}

// ---------------------------------------------------------------------------
// 6. `Options::transaction_keys_inline` — the one knob on the buffering path.
// ---------------------------------------------------------------------------

/// `Transaction::put` at three values of `Options::transaction_keys_inline`.
///
/// Past that many entries the write buffer builds a `HopscotchMap` index over
/// itself, and `TxnBuffer::insert` then clones the key a *second* time to key
/// the index: `let index_key = indexed.map(|_| key.clone())`
/// (`src/txn_buffer.rs:129`) plus the `spill.insert` at `src/txn_buffer.rs:151`.
/// The default is 32 (`DEFAULT_TRANSACTION_KEYS_INLINE`,
/// `src/options.rs:13`), so every transaction above 32 keys pays it.
///
/// A blind-write transaction never reads its own buffer, so for this workload
/// the index is pure cost and `0` — which means "never index, always walk" —
/// should be the cheapest. A value above the transaction size should match it.
/// If the three are flat, the index is not where the buffering cost is.
fn bench_keys_inline(c: &mut Criterion, lane: Lane, label: &str, value: &[u8]) {
    let ks = keys(N);

    let mut g = c.benchmark_group(format!("TxnKeysInline/{}/{label}", lane.tag()));
    g.throughput(Throughput::Elements(N));
    tune(&mut g, per_iter_bytes(N, value));
    for inline in [0usize, 32, 128] {
        let dir = TempDir::new().expect("tempdir");
        let db = OptimisticTransactionDb::open(
            dir.path(),
            Options {
                transaction_keys_inline: inline,
                ..lane.options()
            },
        )
        .expect("open");
        g.bench_function(BenchmarkId::new("put-phase", inline), |b| {
            b.iter(|| {
                let txn = db.begin_transaction_with(IsolationLevel::SnapshotIsolation);
                for k in &ks {
                    txn.put(k, value).expect("txn put");
                }
                txn.rollback();
            })
        });
        g.bench_function(BenchmarkId::new("whole", inline), |b| {
            b.iter(|| {
                let txn = db.begin_transaction_with(IsolationLevel::SnapshotIsolation);
                for k in &ks {
                    txn.put(k, value).expect("txn put");
                }
                txn.commit().expect("commit");
            })
        });
    }
    g.finish();
}

// ---------------------------------------------------------------------------
// Driver
// ---------------------------------------------------------------------------

fn all(c: &mut Criterion) {
    for lane in [Lane::Mem, Lane::Disk] {
        for (label, vsize) in SIZES {
            let value = vec![0xABu8; vsize];

            // The value-size discriminator: the whole split, the direct put and
            // the batch, at both sizes.
            bench_direct_put(c, lane, label, &value, N);
            bench_split(c, lane, label, &value, N);
            bench_batch(c, lane, label, &value, N);

            if label == "64B" {
                // Shape and knob sweeps at 64 B only; the size axis above
                // already covers the per-byte question and these are long.
                bench_batch(c, lane, label, &value, 1_000);
                bench_isolation(c, lane, label, &value);
                bench_concurrency_mode(c, lane, label, &value);
                bench_sizes(c, lane, label, &value);
                bench_keys_inline(c, lane, label, &value);
                bench_cold_validation(c, lane, label, &value);
            }
        }
    }
}

criterion_group!(txncost, all);
criterion_main!(txncost);
