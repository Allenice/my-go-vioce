use std::io::Write;
use std::process::{Command, Stdio};

#[test]
fn persistent_process_reports_errors_then_shutdown_without_hardware() {
    let mut child = Command::new(env!("CARGO_BIN_EXE_FirmwareHelper"))
        .stdin(Stdio::piped())
        .stdout(Stdio::piped())
        .stderr(Stdio::piped())
        .spawn()
        .unwrap();
    let mut input = child.stdin.take().unwrap();
    input.write_all(b"not json\n").unwrap();
    input.write_all(&vec![b'x'; 17000]).unwrap();
    input.write_all(b"\n{\"type\":\"install\",\"port\":\"/dev/cu.fake\",\"firmwarePath\":\"relative\",\"manifestPath\":\"relative\"}\n{\"type\":\"shutdown\"}\n").unwrap();
    drop(input);
    let result = child.wait_with_output().unwrap();
    assert!(result.status.success());
    assert!(result.stderr.is_empty());
    let events: Vec<serde_json::Value> = String::from_utf8(result.stdout)
        .unwrap()
        .lines()
        .map(|s| serde_json::from_str(s).unwrap())
        .collect();
    assert_eq!(events.len(), 5);
    assert_eq!(events[0]["type"], "ready");
    assert_eq!(events[1]["code"], "INVALID_REQUEST");
    assert_eq!(events[2]["code"], "REQUEST_TOO_LARGE");
    assert_eq!(events[3]["stage"], "preflight");
    assert_eq!(events[4]["code"], "INVALID_PATH");
    assert!(events.iter().all(|e| e["type"] != "complete"));
}
