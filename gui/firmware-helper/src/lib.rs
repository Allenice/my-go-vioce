use md5::{Digest, Md5};
use serde::{Deserialize, Serialize};
use sha2::Sha256;
use std::io::{self, BufRead, Read, Write};
use std::path::{Component, Path};

pub const APP_OFFSET: u32 = 0x10000;
pub const APP_MAX: u32 = 0x300000;
pub const MARKER: &str = "UP held: booting permanent recovery";
pub const LINE_LIMIT: usize = 16 * 1024;

#[derive(Debug, Clone, Copy, PartialEq, Eq)]
pub struct Failure(pub &'static str, pub &'static str);
pub type Result<T> = std::result::Result<T, Failure>;

#[derive(Debug, Deserialize)]
#[serde(tag = "type", rename_all = "camelCase", deny_unknown_fields)]
pub enum Request {
    Scan {},
    Install {
        port: String,
        #[serde(rename = "firmwarePath")]
        firmware_path: String,
        #[serde(rename = "manifestPath")]
        manifest_path: String,
    },
    Shutdown {},
}

#[derive(Debug, Serialize)]
#[serde(tag = "type", rename_all = "camelCase")]
pub enum Event {
    Ready,
    Ports {
        devices: Vec<Device>,
    },
    Progress {
        stage: Stage,
        percent: u8,
        message: &'static str,
    },
    Complete,
    Error {
        code: &'static str,
        message: &'static str,
        recoverable: bool,
    },
}
#[derive(Debug, Clone, Copy, Serialize)]
#[serde(rename_all = "lowercase")]
pub enum Stage {
    Preflight,
    Writing,
    Verifying,
    Restarting,
}
#[derive(Debug, Serialize)]
pub struct Device {
    pub id: String,
    pub name: &'static str,
}
pub fn emit(out: &mut impl Write, event: &Event) -> io::Result<()> {
    serde_json::to_writer(&mut *out, event)?;
    out.write_all(b"\n")?;
    out.flush()
}

// Drain an overlong line without allocating more than the protocol bound.
pub fn next_line(input: &mut impl BufRead) -> io::Result<Option<Result<Vec<u8>>>> {
    let mut line = Vec::new();
    let mut oversized = false;
    loop {
        let buf = input.fill_buf()?;
        if buf.is_empty() {
            return if oversized {
                Ok(Some(Err(Failure(
                    "REQUEST_TOO_LARGE",
                    "Request exceeds 16 KiB",
                ))))
            } else if line.is_empty() {
                Ok(None)
            } else {
                Ok(Some(Err(Failure(
                    "INVALID_REQUEST",
                    "Request must end with a newline",
                ))))
            };
        }
        let n = buf
            .iter()
            .position(|&b| b == b'\n')
            .map_or(buf.len(), |i| i + 1);
        let ended = buf[n - 1] == b'\n';
        if line.len() + n > LINE_LIMIT {
            oversized = true;
        }
        if !oversized {
            line.extend_from_slice(&buf[..n]);
        }
        input.consume(n);
        if ended {
            return Ok(Some(if oversized {
                Err(Failure("REQUEST_TOO_LARGE", "Request exceeds 16 KiB"))
            } else {
                Ok(line)
            }));
        }
    }
}

pub fn allowed_port(path: &str, vid: u16, pid: u16) -> bool {
    vid == 0x303a
        && pid == 0x1001
        && path.strip_prefix("/dev/cu.").is_some_and(|s| {
            !s.is_empty()
                && s.bytes()
                    .all(|b| b.is_ascii_alphanumeric() || b"._-".contains(&b))
        })
}
pub fn read_file(path: &str, max: usize) -> Result<Vec<u8>> {
    let p = Path::new(path);
    if !p.is_absolute() || p.components().any(|c| matches!(c, Component::ParentDir)) {
        return Err(Failure(
            "INVALID_PATH",
            "Expected an absolute file path without parent traversal",
        ));
    }
    let file = std::fs::File::open(p)
        .map_err(|_| Failure("FILE_READ_FAILED", "Cannot open input file"))?;
    let meta = file
        .metadata()
        .map_err(|_| Failure("FILE_READ_FAILED", "Cannot inspect input file"))?;
    if !meta.is_file() || meta.len() > max as u64 {
        return Err(Failure(
            "INVALID_FILE",
            "Input must be a bounded regular file",
        ));
    }
    let mut bytes = Vec::new();
    file.take(max as u64 + 1)
        .read_to_end(&mut bytes)
        .map_err(|_| Failure("FILE_READ_FAILED", "Cannot read input file"))?;
    if bytes.len() > max {
        return Err(Failure("INVALID_FILE", "Input exceeds size limit"));
    }
    Ok(bytes)
}

#[derive(Debug, Deserialize)]
#[serde(rename_all = "camelCase", deny_unknown_fields)]
pub struct Manifest {
    pub schema_version: u32,
    pub product: String,
    pub chip: String,
    pub flash_size: u32,
    pub app_offset: u32,
    pub app_max_size: u32,
    pub app_size: u32,
    pub app_sha256: String,
    pub partition_table_offset: u32,
    pub partition_table_size: u32,
    pub partitions: Vec<Partition>,
    pub recovery_marker: String,
}
#[derive(Debug, Clone, Deserialize, PartialEq, Eq)]
#[serde(deny_unknown_fields)]
pub struct Partition {
    pub name: String,
    #[serde(rename = "type")]
    pub kind: u8,
    pub subtype: u8,
    pub offset: u32,
    pub size: u32,
}
fn layout_error() -> Failure {
    Failure(
        "INVALID_LAYOUT",
        "Partition layout does not match the supported Go Voice AI Passport layout",
    )
}
pub fn validate_partitions(parts: &[Partition]) -> Result<()> {
    let required = [
        ("nvs", 1, 2, 0x9000, 0x6000),
        ("phy_init", 1, 1, 0xf000, 0x1000),
        ("factory", 0, 0, APP_OFFSET, APP_MAX),
        ("cardid", 1, 2, 0x356000, 0x4000),
        ("recovery", 0, 0x20, 0x700000, 0x100000),
    ];
    if parts.len() != required.len() {
        return Err(layout_error());
    }
    for (name, kind, subtype, offset, size) in required {
        if parts
            .iter()
            .filter(|p| {
                p.name == name
                    && p.kind == kind
                    && p.subtype == subtype
                    && p.offset == offset
                    && p.size == size
            })
            .count()
            != 1
        {
            return Err(layout_error());
        }
    }
    Ok(())
}
pub fn validate_manifest(manifest: &Manifest, firmware: &[u8]) -> Result<()> {
    let m = manifest;
    if m.schema_version != 1
        || m.product != "Go Voice"
        || m.chip != "esp32c3"
        || m.flash_size != 0x800000
        || m.app_offset != APP_OFFSET
        || m.app_max_size != APP_MAX
        || m.partition_table_offset != 0x8000
        || m.partition_table_size != 0xc00
        || m.recovery_marker != MARKER
    {
        return Err(Failure(
            "INVALID_MANIFEST",
            "Manifest does not match the supported product contract",
        ));
    }
    validate_partitions(&m.partitions)?;
    let erase_end = m
        .app_size
        .checked_add(4095)
        .map(|n| n & !4095)
        .and_then(|n| APP_OFFSET.checked_add(n));
    if m.app_size == 0
        || m.app_size > APP_MAX
        || erase_end.is_none_or(|n| n > 0x356000)
        || firmware.len() != m.app_size as usize
        || firmware.first() != Some(&0xe9)
    {
        return Err(Failure(
            "INVALID_IMAGE",
            "Invalid ESP application image or unsafe image size",
        ));
    }
    if m.app_sha256.len() != 64
        || !m.app_sha256.bytes().all(|b| b.is_ascii_hexdigit())
        || !format!("{:x}", Sha256::digest(firmware)).eq_ignore_ascii_case(&m.app_sha256)
    {
        return Err(Failure(
            "HASH_MISMATCH",
            "Firmware SHA-256 does not match manifest",
        ));
    }
    Ok(())
}

pub fn parse_partition_table(data: &[u8]) -> Result<Vec<Partition>> {
    if data.len() != 4096 {
        return Err(layout_error());
    }
    let mut parts = Vec::new();
    for (index, entry) in data.as_chunks::<32>().0.iter().enumerate() {
        if entry[..2] == [0xeb, 0xeb] {
            if entry[2..16] != [0xff; 14]
                || Md5::digest(&data[..index * 32])[..] != entry[16..]
                || data[(index + 1) * 32..].iter().any(|b| *b != 0xff)
            {
                return Err(layout_error());
            }
            validate_partitions(&parts)?;
            return Ok(parts);
        }
        if entry[..2] != [0xaa, 0x50] || entry[28..32] != [0; 4] {
            return Err(layout_error());
        }
        let label = &entry[12..28];
        let end = label.iter().position(|b| *b == 0).unwrap_or(16);
        if label[end..].iter().any(|b| *b != 0) {
            return Err(layout_error());
        }
        let name = std::str::from_utf8(&label[..end])
            .map_err(|_| layout_error())?
            .to_string();
        parts.push(Partition {
            name,
            kind: entry[2],
            subtype: entry[3],
            offset: u32::from_le_bytes(entry[4..8].try_into().unwrap()),
            size: u32::from_le_bytes(entry[8..12].try_into().unwrap()),
        });
    }
    Err(layout_error())
}

/// Bytes one ROM READ_FLASH_SLOW command returns.
pub const ROM_READ_BLOCK: usize = 64;

/// Extracts the flash bytes of one ROM READ_FLASH_SLOW response. espflash keeps
/// the ROM status trailer (2 bytes on ESP32, 4 on ESP32-C3 and later) after
/// the 64 data bytes; espflash has already rejected a non-zero status. Any other
/// length is refused rather than guessed.
pub fn rom_read_payload(mut response: Vec<u8>) -> Result<Vec<u8>> {
    if ![0, 2, 4].contains(&response.len().wrapping_sub(ROM_READ_BLOCK)) {
        return Err(Failure(
            "PREFLIGHT_READ_UNAVAILABLE",
            "Incomplete ROM read response",
        ));
    }
    response.truncate(ROM_READ_BLOCK);
    Ok(response)
}

pub fn validate_security(
    flags: u32,
    crypt_count: u8,
    chip_id: Option<u32>,
    secure_download: bool,
) -> Result<()> {
    // ESP32-C3 image chip ID is 5. Reject unknown flags and any encryption history conservatively.
    if secure_download || flags != 0 || crypt_count != 0 || chip_id != Some(5) {
        return Err(Failure(
            "UNSAFE_SECURITY_STATE",
            "Cannot confirm an unrestricted, unencrypted ESP32-C3",
        ));
    }
    Ok(())
}
