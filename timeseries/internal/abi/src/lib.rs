use ::safer_ffi::prelude::*;
use tcql;
use chrono::{DateTime, Utc};

#[derive_ReprC]
#[repr(C)]
#[derive(Debug, Clone)]
pub struct Store {
    pub location: safer_ffi::String,
    pub start_ms: u64,
    pub resolution_ms: u64,
}

#[ffi_export]
pub fn create_db(
    db_location: safer_ffi::String,
    array_size: repr_c::Vec<u64>,
) {
    tcql::create_db(db_location.to_string().as_str(), &array_size).unwrap()
}

#[ffi_export]
pub fn read_value(
    store: Store,
    location_ms: repr_c::Vec<u64>,
) -> u64 {
    let location_dt = to_tcql_loc(location_ms);

    tcql::read_value(&store.to_tql_store(), &location_dt).unwrap()
}

#[ffi_export]
pub fn write_value(
    store: Store,
    location_ms: repr_c::Vec<u64>,
    value: u64,
) {
    let location_dt = to_tcql_loc(location_ms);

    tcql::write_value(&store.to_tql_store(), &location_dt, value).unwrap()
}

fn ts_from_ms(ms: u64) -> DateTime::<Utc> {
    let seconds = ms / 1000;
    let nano_seconds = (ms - (seconds*1000))*1000;

    DateTime::from_timestamp(seconds as i64, nano_seconds as u32).unwrap()//todo
}

fn to_tcql_loc(location_ms: repr_c::Vec<u64>) -> Vec<DateTime<Utc>> {
    let mut location_dt = Vec::new();
    for location_ms in location_ms.iter(){
        location_dt.push(ts_from_ms(*location_ms));
    }

    location_dt
}

impl Store {
    fn to_tql_store(&self) -> tcql::Store {
        tcql::Store{
            location: self.location.to_string(),
            start: ts_from_ms(self.start_ms),
            resolution_ms: self.resolution_ms,
        }
    }
}

// The following function is only necessary for the header generation.
#[cfg(feature = "headers")] // c.f. the `Cargo.toml` section
pub fn generate_headers() -> ::std::io::Result<()> {
    ::safer_ffi::headers::builder()
        .to_file("./target/abi.h")?
        .generate()
}
