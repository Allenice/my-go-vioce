#pragma once

#include <stdbool.h>

// iot_button's long-press tick math is compiled from CONFIG_BUTTON_PERIOD_TIME_MS
// (20 ms). Slow scanning is a BSP overlay used only while every key is up.
// Recording keeps the screen lit, so screen_off already excludes capture; the
// capture_active flag is a safety net if that idle policy changes.

static inline bool button_scan_idle(bool screen_off, bool capture_active)
{
    return screen_off && !capture_active;
}
