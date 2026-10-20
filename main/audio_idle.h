#pragma once

#include <stdbool.h>
#include <stdint.h>

// After capture stops, pause I2S first so a quick PTT is a DMA resume. If the
// microphone stays idle this long, fully close the codec to drop analog bias.
// The next set_format() reopens it and may show the measured ADC startup
// transient. Five seconds is long enough that a follow-up utterance while the
// result is still on screen usually hits the warm paused path.
#define AUDIO_IDLE_SUSPEND_US (5LL * 1000 * 1000)

static inline bool audio_idle_should_suspend(bool recording, int64_t idle_us)
{
    return !recording && idle_us >= AUDIO_IDLE_SUSPEND_US;
}
