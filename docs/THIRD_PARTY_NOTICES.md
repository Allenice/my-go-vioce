<p align="right">
  <a href="THIRD_PARTY_NOTICES.zh_CN.md">简体中文</a> · <strong>English</strong>
</p>

# Third-Party Notices

This project is based on
[FoloToy/ai-passport](https://github.com/FoloToy/ai-passport), distributed under
the MIT License with `Copyright (c) 2026 FoloToy`. The full upstream notice is
retained in the repository's [`LICENSE`](../LICENSE).

ESP-IDF Managed Component versions are pinned in
[`dependencies.lock`](../dependencies.lock). The source distributions fetched
by ESP-IDF contain the authoritative license and copyright files and must retain
them when redistributed.

| Component | Pinned version | Primary license in its distribution |
| --- | ---: | --- |
| Espressif ESP-IDF | 5.5.3 | Apache License 2.0, with third-party exceptions documented by ESP-IDF |
| `espressif/button` | 4.2.0 | Apache License 2.0 |
| `espressif/cmake_utilities` | 1.1.1 | Apache License 2.0 |
| `espressif/esp_codec_dev` | 1.6.2 | Apache License 2.0, with included-device notices |
| `espressif/esp_lvgl_port` | 2.9.0 | Apache License 2.0 |
| `lvgl/lvgl` | 9.5.0 | MIT License, with additional bundled third-party notices |

This summary is provided for convenience and does not replace the license files
shipped with each dependency. If dependency source or compiled firmware is
redistributed, include the applicable licenses, copyright statements, notices,
and any component-specific third-party files from the exact resolved versions.

The Vokie name and symbol are not third-party software dependencies and are not
covered by MIT. Their separate redistribution terms are in
[`LICENSES/Vokie-Brand-Asset.txt`](../LICENSES/Vokie-Brand-Asset.txt).

The unused upstream Barlow Condensed Bold font source and subset have been removed. The historical [font license](../LICENSES/Barlow-OFL.txt), Copyright 2017 The Barlow Project Authors, is retained for attribution. Go Voice uses LVGL Montserrat 20. See [Assets](../assets/README.md#historical-vokie-title-font).

Go Voice imports the Vokie firmware at commit `ce26ea043251102155e54bc9503eb8133d5cafa8`; the FoloToy MIT attribution remains intact. Its proprietary symbol is removed from the firmware runtime assets. The Go Voice PNG is user-provided artwork, not a recolored Vokie symbol; this notice grants no independent trademark or artwork rights.

The reference startup WAV and its 16 kHz PCM derivative are retained unchanged from that Vokie commit. Upstream reports creator permission for its firmware, but supplies no standalone sound license. That statement does not establish permission for Go Voice redistribution: confirm independent authorization before releasing source or firmware containing this audio. See [asset provenance](../assets/README.md#vokie-startup-sound).
