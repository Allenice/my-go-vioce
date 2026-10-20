#!/usr/bin/env python3
"""Check the Go Voice wire identity and static LVGL asset without ESP-IDF."""
import hashlib
from pathlib import Path
import re
import unittest
import uuid

ROOT = Path(__file__).resolve().parents[1]


class GoVoiceContractTests(unittest.TestCase):
    def test_uuid_byte_order_and_consumers(self):
        firmware = (ROOT / 'main/vokie_ble.c').read_text()
        consumers = [ROOT / path for path in (
            'docs/ai-passport-ble-protocol.md',
            'docs/ai-passport-ble-protocol.zh_CN.md',
            'gui/internal/protocol/control.go',
            'gui/macos/BridgeCore/VoiceContract.swift',
        )]
        for number, name in enumerate(('SERVICE', 'CONTROL', 'AUDIO', 'INFO'), 1):
            expected = f'676f000{number}-766f-6963-652d-617564696f00'
            line = re.search(rf'^#define {name}_BYTES (.+)$', firmware, re.M).group(1)
            wire = bytes(int(value, 16) for value in line.split(','))
            self.assertEqual(str(uuid.UUID(bytes=wire[::-1])), expected)
            for path in consumers:
                self.assertIn(expected, path.read_text(), str(path))
                self.assertNotIn('7f0e000', path.read_text(), str(path))
        self.assertIn('DEVICE_NAME = "Go Voice"', firmware)

    def test_logo_storage_and_wiring(self):
        asset = (ROOT / 'assets/brand/go_voice_logo_asset.h').read_text()
        data = asset.split('s_go_voice_logo_data[8192] = {', 1)[1]
        pixels = bytes(int(value, 16) for value in re.findall(r'0x([0-9a-f]{2})', data))
        self.assertEqual(len(pixels), 64 * 64 * 2)
        source = (ROOT / 'assets/brand/go-voice-logo.png').read_bytes()
        self.assertIn(hashlib.sha256(source).hexdigest(), asset)
        # Count RGB565 colors, not distinct byte values: a monochrome
        # foreground composited onto the UI background is valid artwork.
        colors = {pixels[offset:offset + 2] for offset in range(0, len(pixels), 2)}
        self.assertGreater(len(colors), 1, 'Logo must not be a solid-color raster')
        self.assertIn(b'\xff\xff', colors, 'Logo must contain its white foreground')
        ui = (ROOT / 'main/ui_status.c').read_text()
        cmake = (ROOT / 'main/CMakeLists.txt').read_text()
        self.assertIn('#include "go_voice_logo_asset.h"', ui)
        self.assertIn('.data = s_go_voice_logo_data', ui)
        self.assertIn('"Go Voice",\n                              &lv_font_montserrat_20', ui)
        self.assertIn('../assets/brand', cmake)
        self.assertNotIn('vokie_title', cmake)
        self.assertFalse((ROOT / 'main/vokie_symbol_asset.h').exists())
        self.assertNotIn('Vokie', ui)


if __name__ == '__main__':
    unittest.main()
