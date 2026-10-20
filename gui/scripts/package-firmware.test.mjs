import { test, expect } from "bun:test";
import { mkdtemp, rm, mkdir, symlink, readdir, copyFile, chmod, access } from "node:fs/promises";
import { tmpdir } from "node:os";
import { join, resolve } from "node:path";
import { createHash } from "node:crypto";
import { inspectFirmware, packageFirmware } from "./package-firmware.mjs";

async function fixture(run) {
  const dir = await mkdtemp(join(tmpdir(), "go-voice-package-test-"));
  try { await run(dir); } finally { await rm(dir, { recursive: true, force: true }); }
}

for (const size of [1, 4095, 4096, 4097, 0x300000]) {
  test(`packages ${size} bytes with exact digest and layout`, () => fixture(async dir => {
    const bytes = Buffer.alloc(size, 0x42); bytes[0] = 0xe9;
    const input = join(dir, "app.bin"), output = join(dir, "firmware");
    await Bun.write(input, bytes);
    const result = await packageFirmware(input, output);
    expect(result.appSha256).toBe(createHash("sha256").update(bytes).digest("hex"));
    expect(result.appSize).toBe(size);
    expect(result.schemaVersion).toBe(1);
    expect(result.firmwareVersion).toMatch(/^\d+\.\d+\.\d+$/);
    expect(result.partitions).toEqual([
      { name: "nvs", type: 1, subtype: 2, offset: 0x9000, size: 0x6000 },
      { name: "phy_init", type: 1, subtype: 1, offset: 0xf000, size: 0x1000 },
      { name: "factory", type: 0, subtype: 0, offset: 0x10000, size: 0x300000 },
      { name: "cardid", type: 1, subtype: 2, offset: 0x356000, size: 0x4000 },
      { name: "recovery", type: 0, subtype: 32, offset: 0x700000, size: 0x100000 },
    ]);
    expect(Math.ceil((result.appOffset + size) / 4096) * 4096).toBeLessThanOrEqual(0x356000);
    expect(await Bun.file(join(output, "manifest.json")).json()).toEqual(result);
    expect(Buffer.from(await Bun.file(join(output, "go-voice-app.bin")).arrayBuffer())).toEqual(bytes);
    expect((await readdir(output)).sort()).toEqual(["go-voice-app.bin", "manifest.json"]);
  }));
}

for (const [name, bytes] of [["empty", Buffer.alloc(0)], ["bad magic", Buffer.from([0])], ["oversize", Buffer.alloc(0x300001, 0xe9)]]) {
  test(`rejects ${name} without replacing existing manifest`, () => fixture(async dir => {
    const input = join(dir, "app.bin"), output = join(dir, "firmware");
    await mkdir(output);
    await Bun.write(join(output, "manifest.json"), "existing");
    await Bun.write(input, bytes);
    await expect(packageFirmware(input, output)).rejects.toThrow();
    expect(await Bun.file(join(output, "manifest.json")).text()).toBe("existing");
    expect(await readdir(output)).toEqual(["manifest.json"]);
  }));
}

test("build cleans generated firmware when Cargo fails and preserves preexisting staging", () => fixture(async dir => {
  const gui = join(dir, "gui"), scripts = join(gui, "scripts"), commands = join(dir, "commands");
  await mkdir(scripts, { recursive: true });
  await mkdir(commands);
  await mkdir(join(gui, "resources"));
  const jiebaModule = join(dir, "gojieba");
  const dictionaries = join(jiebaModule, "deps/cppjieba/dict");
  await mkdir(dictionaries, { recursive: true });
  for (const name of ["jieba.dict.utf8", "hmm_model.utf8", "user.dict.utf8", "idf.utf8", "stop_words.utf8"]) {
    await Bun.write(join(dictionaries, name), "fixture dictionary");
  }
  await mkdir(join(dir, "build"));
  await Bun.write(join(dir, "build/FoloToy-AI-Passport.bin"), Buffer.from([0xe9]));
  await mkdir(join(dir, "main"), { recursive: true });
  await copyFile(resolve(import.meta.dir, "../../main/fw_version.h"), join(dir, "main/fw_version.h"));
  for (const name of ["build.sh", "package-firmware.mjs"]) await copyFile(resolve(import.meta.dir, name), join(scripts, name));
  await Bun.write(join(commands, "uname"), "#!/bin/sh\necho arm64\n");
  await Bun.write(join(commands, "cargo"), "#!/bin/sh\nexit 23\n");
  await Bun.write(join(commands, "go"), '#!/bin/sh\n[ "$1" != list ] || echo "$GOVOICE_TEST_JIEBA_MODULE"\nexit 0\n');
  for (const name of ["uname", "cargo", "go"]) await chmod(join(commands, name), 0o755);
  const build = () => Bun.spawn(["/bin/bash", join(scripts, "build.sh")], {
    env: { ...process.env, PATH: `${commands}:${process.env.PATH}`, WAKU_BUN_EXECUTABLE: process.execPath, GOVOICE_TEST_JIEBA_MODULE: jiebaModule },
    stdout: "ignore", stderr: "ignore",
  }).exited;
  expect(await build()).toBe(23);
  await expect(access(join(gui, "firmware"))).rejects.toThrow();
  await expect(access(join(gui, "resources/jieba"))).rejects.toThrow();
  await mkdir(join(gui, "resources/jieba"));
  await Bun.write(join(gui, "resources/jieba/owned.txt"), "keep dictionary");
  expect(await build()).toBe(1);
  expect(await Bun.file(join(gui, "resources/jieba/owned.txt")).text()).toBe("keep dictionary");
  await rm(join(gui, "resources/jieba"), { recursive: true });
  await mkdir(join(gui, "firmware"));
  await Bun.write(join(gui, "firmware/owned.txt"), "keep");
  expect(await build()).toBe(1);
  expect(await Bun.file(join(gui, "firmware/owned.txt")).text()).toBe("keep");
}));

test("packaged resource verifier rejects tampered manifest and image", () => fixture(async dir => {
  const input = join(dir, "app.bin"), output = join(dir, "firmware");
  await Bun.write(input, Buffer.from([0xe9, 1, 2]));
  await packageFirmware(input, output);
  const verify = () => Bun.spawn([process.execPath, resolve(import.meta.dir, "verify-packaged-firmware.mjs"), dir], { stdout: "ignore", stderr: "ignore" }).exited;
  expect(await verify()).toBe(0);
  const manifestPath = join(output, "manifest.json");
  const manifest = await Bun.file(manifestPath).json();
  await Bun.write(manifestPath, JSON.stringify({ ...manifest, appSha256: "0".repeat(64) }));
  expect(await verify()).not.toBe(0);
  await Bun.write(manifestPath, JSON.stringify(manifest));
  await Bun.write(join(output, "go-voice-app.bin"), Buffer.from([0xe9, 3, 4]));
  expect(await verify()).not.toBe(0);
}));

test("rejects missing files, directories and symlinks", () => fixture(async dir => {
  await expect(inspectFirmware(join(dir, "missing"))).rejects.toThrow();
  await expect(inspectFirmware(dir)).rejects.toThrow();
  const input = join(dir, "app.bin"), link = join(dir, "link.bin");
  await Bun.write(input, Buffer.from([0xe9]));
  await symlink(input, link);
  await expect(inspectFirmware(link)).rejects.toThrow();
}));
