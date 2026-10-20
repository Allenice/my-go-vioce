import { constants } from "node:fs";
import { mkdir, open, rename, rm } from "node:fs/promises";
import { dirname, resolve } from "node:path";
import { createHash, randomUUID } from "node:crypto";

const versionHeader = resolve(dirname(import.meta.path), "../../main/fw_version.h");

export async function readFirmwareVersion(path = versionHeader) {
  const text = await Bun.file(path).text();
  const match = text.match(/^#define GO_VOICE_FW_VERSION "(\d+\.\d+\.\d+)"$/m);
  if (!match) throw new Error("Firmware version missing from fw_version.h");
  return match[1];
}

export const layout = {
  schemaVersion: 1, product: "Go Voice", chip: "esp32c3", flashSize: 8388608,
  appOffset: 65536, appMaxSize: 3145728,
  partitionTableOffset: 32768, partitionTableSize: 3072,
  partitions: [
    { name: "nvs", type: 1, subtype: 2, offset: 36864, size: 24576 },
    { name: "phy_init", type: 1, subtype: 1, offset: 61440, size: 4096 },
    { name: "factory", type: 0, subtype: 0, offset: 65536, size: 3145728 },
    { name: "cardid", type: 1, subtype: 2, offset: 3497984, size: 16384 },
    { name: "recovery", type: 0, subtype: 32, offset: 7340032, size: 1048576 },
  ],
  recoveryMarker: "UP held: booting permanent recovery",
};

export async function inspectFirmware(input, options = {}) {
  // O_NOFOLLOW rejects symlinks; O_NONBLOCK prevents a FIFO from blocking before fstat.
  const file = await open(input, constants.O_RDONLY | constants.O_NOFOLLOW | constants.O_NONBLOCK);
  try {
    const stat = await file.stat();
    if (!stat.isFile()) throw new Error("Firmware must be a regular file");
    if (stat.size === 0 || stat.size > layout.appMaxSize) throw new Error("Firmware size must be 1..0x300000");
    const eraseEnd = Math.ceil((layout.appOffset + stat.size) / 4096) * 4096;
    const cardid = layout.partitions.find(partition => partition.name === "cardid");
    if (!cardid || eraseEnd > cardid.offset) throw new Error("Firmware erase range overlaps cardid");
    const bytes = await file.readFile();
    if (bytes.length !== stat.size) throw new Error("Firmware changed while reading");
    if (bytes[0] !== 0xe9) throw new Error("Firmware must start with ESP image magic 0xE9");
    const firmwareVersion = options.firmwareVersion ?? await readFirmwareVersion();
    return { bytes, manifest: { ...layout, firmwareVersion, appSize: bytes.length, appSha256: createHash("sha256").update(bytes).digest("hex") } };
  } finally {
    await file.close();
  }
}

export async function packageFirmware(input, output, options = {}) {
  const { bytes, manifest } = await inspectFirmware(input, options);
  await mkdir(output, { recursive: true });
  const manifestPath = resolve(output, "manifest.json");
  const temporary = `${manifestPath}.${randomUUID()}.tmp`;
  try {
    await Bun.write(resolve(output, "go-voice-app.bin"), bytes);
    await Bun.write(temporary, JSON.stringify(manifest, null, 2) + "\n");
    await rename(temporary, manifestPath);
  } finally {
    await rm(temporary, { force: true });
  }
  return manifest;
}

if (import.meta.main) {
  const root = resolve(dirname(import.meta.path), "../..");
  const input = process.argv[2] ?? resolve(root, "build/FoloToy-AI-Passport.bin");
  const output = process.argv[3] ?? resolve(root, "gui/firmware");
  try {
    await packageFirmware(input, output);
  } catch (error) {
    console.error(`Firmware packaging failed: ${error.message}`);
    process.exitCode = 1;
  }
}
