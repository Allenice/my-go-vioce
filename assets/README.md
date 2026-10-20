<p align="right">
  <a href="README.zh_CN.md">简体中文</a> · <strong>English</strong>
</p>

# Assets

## Go Voice logo and title

- Source: [brand/go-voice-gopher-source.png](brand/go-voice-gopher-source.png), the original AI-generated artwork created from the user's reference and request. A rounded Gopher with a curious tilt, expressive eyes, small front teeth and tucked paws is paired with a lower-right microphone. The final artwork uses a white foreground on transparency: original white areas stay white, and original black areas become transparent, including the background and facial cutouts. Antialiased edges retain partial alpha. The derived [brand/go-voice-logo.png](brand/go-voice-logo.png) is 1024x1024 RGBA and remains transparent for firmware generation. The desktop icon `gui/resources/icon.png` uses the same white artwork on an opaque black rounded tile, with transparency only outside the tile. No Vokie symbol is used.
- Generate all logo assets with `"$WAKU_BUN_EXECUTABLE" tools/generate-go-voice-logo.mjs` (bundled Bun with `Bun.Image` and FFmpeg required). Bun.Image resizes the original to 1024px; FFmpeg composites onto black, normalizes grayscale to remove faint texture while retaining antialiased edges, and applies a rounded tile with a 48px inset and 216px radius. It then overlays [brand/go-voice-microphone.png](brand/go-voice-microphone.png), preserving the mascot, and converts the result to white RGB with alpha equal to the previous white coverage (`new_alpha = old_alpha * old_gray / 255`). This removes the black tile and makes facial lines transparent. The microphone's editable source is [brand/go-voice-microphone.svg](brand/go-voice-microphone.svg); regenerate its transparent 1024px PNG on macOS with `swift tools/render-go-voice-logo.swift assets/brand/go-voice-microphone.svg assets/brand/go-voice-microphone.png` before running the main generator. Bun.Image resizes the finished icon to 64x64; FFmpeg composites all transparent areas onto the existing `#0B1018` screen background and emits RGB565 little-endian [go_voice_logo_asset.h](brand/go_voice_logo_asset.h). The temporary directory is removed in `finally`; the original artwork is preserved, and derived PNGs and the header are regenerated. The header records the PNG SHA-256.
- Generate the desktop icon alone with `"$WAKU_BUN_EXECUTABLE" tools/generate-go-voice-app-icon.mjs`. It composites the transparent white artwork onto black and applies a 48px inset and 216px corner radius at 1024px. The complete logo generator also calls this step, so regenerating firmware assets does not restore a transparent desktop icon. The app-only command never rewrites the logo PNG, firmware header or firmware binary.
- `main/ui_status.c` includes this header through the `main/CMakeLists.txt` brand include path. The raster occupies 8192 constant bytes, the same as the old image; it needs no runtime image decoder or resize buffer. The original high-resolution image is not embedded.
- The title uses the already-enabled LVGL Montserrat 20 font, which includes all `Go Voice` characters. The existing title position, width, alignment and clipping remain unchanged; no new font or heap allocation is introduced.
- The user-directed branding is used for this project; no independent redistribution license is asserted here. See [third-party notices](../docs/THIRD_PARTY_NOTICES.md).

## Vokie startup sound

- Source: the reference `v2-blip-octave-tail.wav` from Vokie commit `ce26ea043251102155e54bc9503eb8133d5cafa8`, saved as [vokie-boot-v2-blip-octave-tail.wav](music/vokie-boot-v2-blip-octave-tail.wav). Upstream reports creator permission for its firmware but provides no standalone license. Confirm independent authorization before releasing Go Voice source or firmware containing the sound.
- Source format: 2 seconds, 44.1 kHz, signed 16-bit mono WAV.
- Firmware asset: [vokie-boot-16k.pcm](music/vokie-boot-16k.pcm), 16 kHz signed 16-bit little-endian mono PCM, 64,000 bytes. No normalization or speed change is applied.
- Integration: `main/CMakeLists.txt` embeds only the PCM in flash. `main/boot_sound.c` uses a 640-byte stack buffer on the existing audio worker, at output volume 55/100. Playback runs once per boot and yields to recording. Silence drains playback, then output is muted and startup microphone samples are discarded. The microphone format and gain stay unchanged; playback failure is logged without preventing voice input.

Regenerate from the repository root:

```bash
ffmpeg -nostdin -i assets/music/vokie-boot-v2-blip-octave-tail.wav \
  -ar 16000 -ac 1 -c:a pcm_s16le -f s16le \
  assets/music/vokie-boot-16k.pcm
```

This directory stores reusable fonts, images, music, and sound effects, organized by asset type.

Keep each asset in the matching subdirectory and document its destination, naming, integration method, and source/license. Do not mix binary assets with Markdown documentation.

## Fonts

Store reusable font files and generated font sources in `fonts/`.

- Use descriptive names that include the family, weight, size, and format when relevant.
- Document the source, license, character range, conversion command, and expected destination.
- Check Flash and internal-RAM impact before adding a font; the ESP32-C3 has no PSRAM.
- Do not commit fonts whose license does not permit redistribution.

## Hardware purchase image

- Source: the user-provided FoloToy WeChat Shop product-sharing image, retained as [images/ai-passport-purchase.jpg](images/ai-passport-purchase.jpg) (1020 x 2208 JPEG).
- Website copy: `website/public/img/ai-passport-purchase.jpg`, copied byte-for-byte to preserve the complete mini-program code. The hardware purchase section links to the full-size image and the [official product introduction](https://ai-passport.folotoy.cn/).
- The image is supplied for this website; no independent redistribution license is asserted. Its price and promotions are a snapshot, not a live quotation; the website directs visitors to the shop for current terms.

## Images

Store reusable source images and generated display assets in `images/`.

- Use descriptive names and document dimensions, pixel format, conversion steps, and destination.
- Prefer formats suitable for the 240 × 320 RGB565 display and account for Flash and internal RAM.
- Preserve editable sources where licensing permits, and record the source and license.
- Never commit device QR secrets, credentials, or personal data in images.

## Music and sound effects

Store reusable music and sound-effect sources in `music/`.

- Document the source, license, sample rate, bit depth, channels, conversion command, and destination.
- Prefer 16 kHz, 16-bit mono PCM when it matches the current BSP audio path.
- Check Flash and internal-RAM cost before embedding audio; stream or chunk long recordings.
- Do not commit media without redistribution permission.

## Historical Vokie title font

The unused Barlow Condensed Bold source and generated LVGL subset have been removed. Go Voice uses LVGL Montserrat 20. The historical [Barlow OFL notice](../LICENSES/Barlow-OFL.txt) is retained for attribution.
