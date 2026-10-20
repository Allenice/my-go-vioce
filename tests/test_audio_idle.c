#include "audio_idle.h"
#include <assert.h>
#include <stdio.h>

int main(void)
{
    assert(AUDIO_IDLE_SUSPEND_US == 5LL * 1000 * 1000);
    assert(!audio_idle_should_suspend(true, AUDIO_IDLE_SUSPEND_US));
    assert(!audio_idle_should_suspend(false, AUDIO_IDLE_SUSPEND_US - 1));
    assert(audio_idle_should_suspend(false, AUDIO_IDLE_SUSPEND_US));
    assert(audio_idle_should_suspend(false, AUDIO_IDLE_SUSPEND_US * 4));
    puts("Audio idle suspend policy tests: PASS");
}
