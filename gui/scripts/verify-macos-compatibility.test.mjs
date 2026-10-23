import { test, expect, beforeAll, afterAll } from "bun:test";
import { mkdtemp, mkdir, copyFile, chmod, rm, writeFile } from "node:fs/promises";
import { tmpdir } from "node:os";
import { join, resolve } from "node:path";

const root = resolve(import.meta.dir, "..");
const minimum = (await Bun.file(join(root, "mygo.json")).json()).macos.minimumSystemVersion;
let fixtures;

function command(args) {
  const result = Bun.spawnSync(args, { stdout: "pipe", stderr: "pipe" });
  if (result.exitCode !== 0) throw new Error(result.stderr.toString());
  return result.stdout.toString();
}

beforeAll(async () => {
  fixtures = await mkdtemp(join(tmpdir(), "go-voice-compatibility-"));
  const source = join(fixtures, "main.c");
  await writeFile(source, "int main(void) { return 0; }\n");
  // Real Mach-O executables exercise the same Apple tools as release checks.
  for (const [name, arch, target] of [
    ["compatible", "arm64", minimum],
    ["too-new", "arm64", `${Number(minimum.split(".")[0]) + 1}.0`],
    ["patch-too-new", "arm64", `${minimum}.1`],
    ["intel", "x86_64", minimum],
  ]) {
    command(["xcrun", "clang", "-arch", arch, `-mmacosx-version-min=${target}`, source, "-o", join(fixtures, name)]);
  }
});

afterAll(async () => {
  if (fixtures) await rm(fixtures, { recursive: true, force: true });
});

async function bundle(path, binary = "compatible", declared = minimum) {
  await mkdir(join(path, "Contents/MacOS"), { recursive: true });
  await copyFile(join(fixtures, binary), join(path, "Contents/MacOS/probe"));
  await chmod(join(path, "Contents/MacOS/probe"), 0o755);
  await writeFile(join(path, "Contents/Info.plist"), `<?xml version="1.0" encoding="UTF-8"?>
<plist version="1.0"><dict>
<key>CFBundleExecutable</key><string>probe</string>
<key>LSMinimumSystemVersion</key><string>${declared}</string>
</dict></plist>\n`);
}

async function check({ main = "compatible", helper = "compatible", declared = minimum, helperMinimum = minimum, missingHelper = false } = {}) {
  const dir = await mkdtemp(join(fixtures, "case-"));
  const app = join(dir, "probe.app");
  await bundle(app, main, declared);
  for (const name of ["BluetoothHelper", "SpeechBridge", "FirmwareHelper"]) {
    if (missingHelper && name === "FirmwareHelper") continue;
    await bundle(join(app, "Contents/Resources/native", `${name}.app`), helper, helperMinimum);
  }
  const result = Bun.spawnSync(["bash", join(root, "scripts/verify-macos-compatibility.sh"), app], { stdout: "pipe", stderr: "pipe" });
  return { code: result.exitCode, output: result.stdout.toString() + result.stderr.toString() };
}

test("accepts arm64 executables built for the declared system with a newer SDK", async () => {
  const result = await check();
  expect(result.code).toBe(0);
  expect(result.output.match(/macOS compatibility:/g)).toHaveLength(4);
});

test("rejects a main executable requiring a newer OS despite a compatible plist", async () => {
  const result = await check({ main: "too-new" });
  expect(result.code).not.toBe(0);
  expect(result.output).toContain("but its Info.plist declares");
});

test("checks every helper for a newer deployment target", async () => {
  const result = await check({ helper: "too-new" });
  expect(result.code).not.toBe(0);
  expect(result.output).toContain("BluetoothHelper.app");
});

test("does not ignore minimum system patch versions", async () => {
  expect((await check({ main: "patch-too-new" })).code).not.toBe(0);
});

test("rejects an Intel-only executable", async () => {
  expect((await check({ main: "intel" })).code).not.toBe(0);
});

test("rejects application metadata inconsistent with the supported minimum", async () => {
  const result = await check({ declared: "27.0" });
  expect(result.code).not.toBe(0);
  expect(result.output).toContain("does not match");
});

test("rejects helpers declaring a newer minimum even when their binary is compatible", async () => {
  const result = await check({ helperMinimum: "27.0" });
  expect(result.code).not.toBe(0);
  expect(result.output).toContain("above the supported");
});

test("fails closed for a missing helper or malformed system version", async () => {
  expect((await check({ missingHelper: true })).code).not.toBe(0);
  expect((await check({ declared: "unknown" })).code).not.toBe(0);
});
