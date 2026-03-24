use ::safer_ffi::prelude::*;

#[ffi_export]
pub fn get(
    _location: repr_c::Vec<u64>,
) -> repr_c::Vec<u8> {
    let mut vec = Vec::new();
    vec.push(1);
    vec.push(2);
    if _location[0] == 1 {
        vec.push(3);
        vec.push(4);
    }
    vec.into()
}

// The following function is only necessary for the header generation.
#[cfg(feature = "headers")] // c.f. the `Cargo.toml` section
pub fn generate_headers() -> ::std::io::Result<()> {
    ::safer_ffi::headers::builder()
        .to_file("./target/abi.h")?
        .generate()
}
