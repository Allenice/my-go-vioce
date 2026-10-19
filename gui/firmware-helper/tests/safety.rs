use firmware_helper::*;
use md5::{Digest, Md5};
use serde_json::json;
use sha2::Sha256;
use std::io::Cursor;

fn manifest(firmware: &[u8]) -> Manifest {
    serde_json::from_value(json!({
        "schemaVersion":1, "product":"Go Voice", "chip":"esp32c3", "flashSize":8388608,
        "appOffset":65536, "appMaxSize":3145728, "appSize":firmware.len(),
        "appSha256":format!("{:x}", Sha256::digest(firmware)),
        "partitionTableOffset":32768, "partitionTableSize":3072, "recoveryMarker":MARKER,
        "partitions":[
            {"name":"nvs","type":1,"subtype":2,"offset":36864,"size":24576},
            {"name":"phy_init","type":1,"subtype":1,"offset":61440,"size":4096},
            {"name":"factory","type":0,"subtype":0,"offset":65536,"size":3145728},
            {"name":"cardid","type":1,"subtype":2,"offset":3497984,"size":16384},
            {"name":"recovery","type":0,"subtype":32,"offset":7340032,"size":1048576}
        ]
    }))
    .unwrap()
}
fn table() -> Vec<u8> {
    let m = manifest(&[0xe9]);
    let mut data = vec![0xff; 4096];
    for (p, e) in m
        .partitions
        .iter()
        .zip(data.as_chunks_mut::<32>().0.iter_mut())
    {
        e.fill(0);
        e[..4].copy_from_slice(&[0xaa, 0x50, p.kind, p.subtype]);
        e[4..8].copy_from_slice(&p.offset.to_le_bytes());
        e[8..12].copy_from_slice(&p.size.to_le_bytes());
        e[12..12 + p.name.len()].copy_from_slice(p.name.as_bytes());
    }
    let digest = Md5::digest(&data[..160]);
    data[160..162].copy_from_slice(&[0xeb, 0xeb]);
    data[176..192].copy_from_slice(&digest);
    data
}
#[test]
fn manifest_accepts_only_safe_images() {
    for size in [1, 4095, 4096, 4097, APP_MAX as usize] {
        let mut firmware = vec![0; size];
        firmware[0] = 0xe9;
        assert!(validate_manifest(&manifest(&firmware), &firmware).is_ok());
    }
    for firmware in [vec![], vec![0], vec![0xe9; APP_MAX as usize + 1]] {
        assert!(validate_manifest(&manifest(&firmware), &firmware).is_err());
    }
    let mut m = manifest(&[0xe9]);
    m.app_size = u32::MAX;
    assert!(validate_manifest(&m, &[0xe9]).is_err());
    m = manifest(&[0xe9]);
    m.app_sha256 = "00".repeat(32);
    assert_eq!(
        validate_manifest(&m, &[0xe9]).unwrap_err().0,
        "HASH_MISMATCH"
    );
    m = manifest(&[0xe9]);
    m.app_size = 2;
    assert!(validate_manifest(&m, &[0xe9]).is_err());
}
#[test]
fn manifest_rejects_contract_changes() {
    let good = [0xe9];
    let mutations: [fn(&mut Manifest); 12] = [
        |m| m.schema_version = 2,
        |m| m.product = "other".into(),
        |m| m.chip = "esp32".into(),
        |m| m.flash_size = 0x400000,
        |m| m.app_offset = 0,
        |m| m.app_max_size += 1,
        |m| m.partition_table_offset = 0x9000,
        |m| m.partition_table_size = 0x1000,
        |m| m.recovery_marker.clear(),
        |m| m.partitions[4].subtype = 0,
        |m| m.partitions[3].offset += 4096,
        |m| m.partitions.push(m.partitions[0].clone()),
    ];
    for mutate in mutations {
        let mut m = manifest(&good);
        mutate(&mut m);
        assert!(validate_manifest(&m, &good).is_err());
    }
}
#[test]
fn partition_table_requires_md5_and_exact_layout() {
    let good = table();
    assert_eq!(
        parse_partition_table(&good).unwrap(),
        manifest(&[0xe9]).partitions
    );
    for at in [0, 2, 3, 4, 8, 12, 28, 160, 162, 176, 192, 4095] {
        let mut bad = good.clone();
        bad[at] ^= 1;
        assert!(parse_partition_table(&bad).is_err(), "corruption at {at}");
    }
    let mut bad = good.clone();
    bad[160..192].fill(0xff);
    assert!(parse_partition_table(&bad).is_err());
    assert!(parse_partition_table(&good[..4095]).is_err());
    // Even a correctly rehashed dangerous layout must fail.
    let mut bad = good.clone();
    bad[4] ^= 1;
    let digest = Md5::digest(&bad[..160]);
    bad[176..192].copy_from_slice(&digest);
    assert!(parse_partition_table(&bad).is_err());
}
#[test]
fn usb_filter_and_paths_reject_untrusted_inputs() {
    assert!(allowed_port("/dev/cu.usbmodem123", 0x303a, 0x1001));
    for path in [
        "/dev/tty.usbmodem123",
        "/dev/cu.",
        "/dev/cu.a/../b",
        "/dev/cu.a\n",
        "cu.usb",
        "/dev/cu.a\0",
    ] {
        assert!(!allowed_port(path, 0x303a, 0x1001));
    }
    assert!(!allowed_port("/dev/cu.usb", 0x303a, 0x1002));
    assert!(!allowed_port("/dev/cu.usb", 0x303b, 0x1001));
    for path in ["relative.bin", "/tmp/../etc/input"] {
        assert_eq!(read_file(path, 1024).unwrap_err().0, "INVALID_PATH");
    }
    assert_eq!(read_file("/dev/null", 1024).unwrap_err().0, "INVALID_FILE");
}
#[test]
fn security_is_fail_closed() {
    assert!(validate_security(0, 0, Some(5), false).is_ok());
    for flags in [1, 2, 4, 1 << 31] {
        assert!(validate_security(flags, 0, Some(5), false).is_err());
    }
    for count in [1, 2, 3, 255] {
        assert!(validate_security(0, count, Some(5), false).is_err());
    }
    assert!(validate_security(0, 0, None, false).is_err());
    assert!(validate_security(0, 0, Some(0), false).is_err());
    assert!(validate_security(0, 0, Some(5), true).is_err());
}
#[test]
fn bounded_ndjson_recovers_after_bad_lines() {
    let mut data = vec![b'x'; LINE_LIMIT * 3];
    data.extend_from_slice(b"\n{\"type\":\"shutdown\"}\n");
    let mut input = Cursor::new(data);
    assert_eq!(
        next_line(&mut input).unwrap().unwrap().unwrap_err().0,
        "REQUEST_TOO_LARGE"
    );
    let line = next_line(&mut input).unwrap().unwrap().unwrap();
    assert!(matches!(
        serde_json::from_slice::<Request>(&line).unwrap(),
        Request::Shutdown {}
    ));
    assert!(next_line(&mut input).unwrap().is_none());
    let mut exact = vec![b' '; LINE_LIMIT];
    exact[LINE_LIMIT - 1] = b'\n';
    assert!(next_line(&mut Cursor::new(exact)).unwrap().unwrap().is_ok());
    assert!(next_line(&mut Cursor::new(b"{}" as &[u8]))
        .unwrap()
        .unwrap()
        .is_err());
}
#[test]
fn requests_and_events_have_stable_private_schema() {
    for input in [
        r#"{"type":"erase"}"#,
        r#"{"type":"scan","extra":1}"#,
        r#"{"type":"install","port":"x"}"#,
    ] {
        assert!(serde_json::from_str::<Request>(input).is_err());
    }
    assert!(matches!(
        serde_json::from_str::<Request>(
            r#"{"type":"install","port":"x","firmwarePath":"/a","manifestPath":"/b"}"#
        )
        .unwrap(),
        Request::Install { .. }
    ));
    let events = [
        Event::Ready,
        Event::Ports {
            devices: vec![Device {
                id: "/dev/cu.usb".into(),
                name: "Go Voice USB Serial/JTAG",
            }],
        },
        Event::Progress {
            stage: Stage::Verifying,
            percent: 50,
            message: "Checking",
        },
        Event::Complete,
        Event::Error {
            code: "INVALID_REQUEST",
            message: "Invalid request",
            recoverable: true,
        },
    ];
    let mut out = Vec::new();
    for event in &events {
        emit(&mut out, event).unwrap();
    }
    let text = String::from_utf8(out).unwrap();
    let lines: Vec<serde_json::Value> = text
        .lines()
        .map(|s| serde_json::from_str(s).unwrap())
        .collect();
    assert_eq!(lines.len(), 5);
    assert_eq!(lines[0], json!({"type":"ready"}));
    assert_eq!(lines[1]["devices"][0].as_object().unwrap().len(), 2);
    assert_eq!(lines[2]["stage"], "verifying");
    assert!(!text.contains("serialNumber"));
    assert_eq!(lines[3], json!({"type":"complete"}));
}
#[test]
fn rom_read_strips_status_trailer_only() {
    // espflash returns the 64 data bytes followed by the ROM status trailer.
    for trailer in [0usize, 2, 4] {
        let mut response: Vec<u8> = (0..64).collect();
        response.extend(std::iter::repeat_n(0, trailer));
        assert_eq!(
            rom_read_payload(response).unwrap(),
            (0..64).collect::<Vec<u8>>()
        );
    }
    for len in [0usize, 32, 63, 65, 67, 69, 128] {
        assert_eq!(
            rom_read_payload(vec![0; len]).unwrap_err().0,
            "PREFLIGHT_READ_UNAVAILABLE"
        );
    }
}
