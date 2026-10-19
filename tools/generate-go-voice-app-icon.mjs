#!/usr/bin/env bun
// Build only the macOS icon; the transparent logo and firmware remain unchanged.
import { copyFileSync, mkdtempSync, rmSync } from 'node:fs';
import { spawnSync } from 'node:child_process';
import { tmpdir } from 'node:os';
import { join, resolve } from 'node:path';

const root = resolve(import.meta.dir, '..');

export async function generateGoVoiceAppIcon() {
  if (typeof Bun.Image !== 'function') {
    throw new Error('Run with $WAKU_BUN_EXECUTABLE: this generator requires Bun.Image.');
  }
  const temporaryDirectory = mkdtempSync(join(tmpdir(), 'go-voice-app-icon-'));
  const output = resolve(root, 'gui/resources/icon.png');
  try {
    const scaled = join(temporaryDirectory, 'scaled.png');
    const composed = join(temporaryDirectory, 'icon.png');
    await new Bun.Image(resolve(root, 'assets/brand/go-voice-logo.png'))
      .resize(1024, 1024).png().write(scaled);
    // Opaque black rounded tile, 48px inset and 216px corner radius.
    // Only the outer margin is transparent; facial cutouts reveal black.
    const alpha = '255*clip(216.5-hypot(max(abs(X-511.5)-248,0),max(abs(Y-511.5)-248,0)),0,1)';
    const result = spawnSync('ffmpeg', [
      '-v', 'error', '-i', scaled,
      '-filter_complex',
      `[0:v]format=rgba[logo];color=c=black:s=1024x1024,format=rgba[bg];` +
      `[bg][logo]overlay=shortest=1:format=auto,format=rgba,` +
      `geq=r='r(X,Y)':g='g(X,Y)':b='b(X,Y)':a='${alpha}',format=rgba[out]`,
      '-map', '[out]', '-frames:v', '1', composed
    ], { encoding: 'utf8' });
    if (result.status !== 0) throw new Error(result.stderr || String(result.error));
    copyFileSync(composed, output);
  } finally {
    rmSync(temporaryDirectory, { recursive: true, force: true });
  }
  console.log(`Generated ${output}: white artwork on a black rounded tile`);
}

if (import.meta.main) await generateGoVoiceAppIcon();
