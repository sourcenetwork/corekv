use cql_u64::U64;
use chrono::{DateTime,Utc};

pub struct Store {
    pub location: String,
    pub start: DateTime::<Utc>,
    pub resolution_ms: u64,
}

pub fn create_db(
    db_location: &str,
    array_size: &[u64],
) -> std::result::Result<(), cql_db::error::Error> {
    cql_db::create_db::<U64>(db_location, array_size)
}

pub fn read_value(
    db: &Store,
    location: &[DateTime::<Utc>], // todo - last index must be time-type, consider typing this (u64 is really a abi thing)
) -> std::result::Result<u64, cql_db::error::Error> {
    let location_ms = [get_index(location[0], db.start, db.resolution_ms)]; // todo - handle other indexes :)
    cql_db::read_value::<U64>(&db.location, &location_ms)
}

pub fn write_value(
    db: &Store,
    location: &[DateTime::<Utc>], // todo - last index must be time-type, consider typing this (u64 is really a abi thing)
    value: u64,
) -> std::result::Result<(), cql_db::error::Error> {
    let location_ms = [get_index(location[0], db.start, db.resolution_ms)]; // todo - handle other indexes :)
    cql_db::write_value::<U64>(&db.location, &location_ms, value)
}

fn get_index(
    time: DateTime::<Utc>,
    start: DateTime::<Utc>,
    resolution_ms: u64,
) -> u64 {
    let ms_from_start = time.timestamp_millis() - start.timestamp_millis();

    // +1 as cql is 1-indexed...
    ((ms_from_start as u64) / resolution_ms)+ 1
}
