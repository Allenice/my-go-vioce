// Derives the macOS menu-bar template icon from the transparent white logo:
// crops to the artwork's alpha bounds (square, with a small margin), scales
// to 36x36 px (18 pt at 2x) and makes it black on transparent, the template
// convention; macOS tints it from the alpha alone.
//
//   "$WAKU_BUN_EXECUTABLE" gui/scripts/generate-tray-icon.mjs
import { spawnSync } from "node:child_process";
import { fileURLToPath } from "node:url";
import path from "node:path";

const root = path.resolve(path.dirname(fileURLToPath(import.meta.url)), "../..");
const source = path.join(root, "assets/brand/go-voice-logo.png");
const target = path.join(root, "gui/resources/tray.png");
const size = 36;

function ffmpeg(args, options = {}) {
  const run = spawnSync("ffmpeg", ["-nostdin", "-v", "error", ...args], { maxBuffer: 1 << 26, ...options });
  if (run.status !== 0) throw new Error(`ffmpeg failed: ${run.stderr}`);
  return run.stdout;
}

const raw = ffmpeg(["-i", source, "-f", "rawvideo", "-pix_fmt", "rgba", "-"]);
const width = 1024;
const height = raw.length / 4 / width;
let x0 = width, y0 = height, x1 = -1, y1 = -1;
for (let y = 0; y < height; y++) {
  for (let x = 0; x < width; x++) {
    if (raw[(y * width + x) * 4 + 3] > 8) {
      x0 = Math.min(x0, x); x1 = Math.max(x1, x);
      y0 = Math.min(y0, y); y1 = Math.max(y1, y);
    }
  }
}
if (x1 < 0) throw new Error("logo has no opaque pixels");
const side = Math.ceil(Math.max(x1 - x0 + 1, y1 - y0 + 1) * 1.04);
const cx = (x0 + x1) / 2, cy = (y0 + y1) / 2;
const left = Math.round(cx - side / 2), top = Math.round(cy - side / 2);
// Pad first so a square crop near an edge stays inside the frame.
const pad = side;
ffmpeg([
  "-y", "-i", source, "-frames:v", "1", "-vf",
  [
    `pad=${width + 2 * pad}:${height + 2 * pad}:${pad}:${pad}:color=0x00000000`,
    `crop=${side}:${side}:${left + pad}:${top + pad}`,
    `scale=${size}:${size}:flags=lanczos`,
    "format=rgba",
    "lutrgb=r=0:g=0:b=0",
  ].join(","),
  target,
]);
console.log(`wrote ${path.relative(root, target)} (${size}x${size}, from bounds ${x0},${y0}-${x1},${y1})`);
