import { resolve } from "node:path";
import { inspectFirmware } from "./package-firmware.mjs";
import { isDeepStrictEqual } from "node:util";

const resources = resolve(process.argv[2] ?? "build/debug/darwin-arm64/go-voice-debug.app/Contents/Resources");
const { manifest: expected } = await inspectFirmware(resolve(resources, "firmware/go-voice-app.bin"));
const actual = await Bun.file(resolve(resources, "firmware/manifest.json")).json();
if (!isDeepStrictEqual(actual, expected)) throw new Error("Bundled firmware manifest does not match image and layout");
console.log("Bundled firmware digest, size and manifest: PASS");
