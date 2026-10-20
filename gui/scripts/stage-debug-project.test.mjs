import { test, expect } from "bun:test";
import { mkdtemp, rm, readlink, lstat } from "node:fs/promises";
import { tmpdir } from "node:os";
import { resolve, join } from "node:path";
import { stageDebugProject } from "./stage-debug-project.mjs";

test("debug packaging isolates the name, identifier and updater without modifying release config", async () => {
  const source = resolve(import.meta.dir, "..");
  const before = await Bun.file(join(source, "mygo.json")).text();
  const staging = await mkdtemp(join(tmpdir(), "go-voice-debug-test-"));
  try {
    await stageDebugProject(source, staging);
    const production = JSON.parse(before);
    const debug = await Bun.file(join(staging, "mygo.json")).json();
    expect(debug.name).toBe("go-voice-debug");
    expect(debug.identifier).toBe("yz.sh.mygo-go-voice.debug");
    expect(debug.version).toBe(production.version);
    expect(debug.updates).toBeUndefined();
    expect(debug.out).toBe(join(source, "build/debug"));
    expect(debug.macos.helperEntitlements).toEqual(production.macos.helperEntitlements);
    expect(await readlink(join(staging, "internal"))).toBe(join(source, "internal"));
    expect((await lstat(join(staging, "resources"))).isDirectory()).toBe(true);
    expect((await lstat(join(staging, "resources/icon.png"))).isFile()).toBe(true);
    expect(await Bun.file(join(source, "mygo.json")).text()).toBe(before);
  } finally {
    await rm(staging, { recursive: true, force: true });
  }
});
