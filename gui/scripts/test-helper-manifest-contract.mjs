import { mkdtemp, rm } from "node:fs/promises";
import { tmpdir } from "node:os";
import { join, resolve } from "node:path";
import { packageFirmware } from "./package-firmware.mjs";

const root = resolve(import.meta.dir, "..");
const helper = resolve(root, "firmware-helper/target/debug/FirmwareHelper");
const directory = await mkdtemp(join(tmpdir(), "go-voice-helper-contract-"));

try {
  const input = join(directory, "app.bin");
  const output = join(directory, "firmware");
  await Bun.write(input, Uint8Array.from([0xe9, 1, 2, 3]));
  await packageFirmware(input, output);

  const child = Bun.spawn([helper], { stdin: "pipe", stdout: "pipe", stderr: "pipe" });
  child.stdin.write(JSON.stringify({
    type: "install",
    port: "/dev/cu.go-voice-contract-does-not-exist",
    firmwarePath: join(output, "go-voice-app.bin"),
    manifestPath: join(output, "manifest.json"),
  }) + "\n");
  child.stdin.write('{"type":"shutdown"}\n');
  child.stdin.end();

  const [status, stdout, stderr] = await Promise.all([
    child.exited,
    new Response(child.stdout).text(),
    new Response(child.stderr).text(),
  ]);
  if (status !== 0 || stderr !== "") throw new Error("FirmwareHelper contract process failed");
  const events = stdout.trim().split(/\n+/).filter(Boolean).map(line => JSON.parse(line));
  if (events[0]?.type !== "ready") throw new Error("FirmwareHelper did not become ready");
  if (!events.some(event => event.type === "progress" && event.stage === "preflight")) {
    throw new Error("FirmwareHelper did not accept the packaged manifest for preflight");
  }
  const error = events.find(event => event.type === "error");
  if (!error || !["PORT_NOT_ALLOWED", "SCAN_FAILED"].includes(error.code)) {
    throw new Error(`Unexpected contract result: ${JSON.stringify(events)}`);
  }
  if (events.some(event => event.type === "complete")) throw new Error("Contract probe must not flash");
  console.log("Bun manifest and Rust FirmwareHelper contract: PASS");
} finally {
  await rm(directory, { recursive: true, force: true });
}
