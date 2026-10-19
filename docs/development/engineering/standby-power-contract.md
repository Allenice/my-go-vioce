<p align="right">
  <a href="standby-power-contract.zh_CN.md">简体中文</a> · <strong>English</strong>
</p>

# Standby power contract

## Design rationale

Standby combines idle I2S pause, event-driven UI work, ESP-IDF automatic light sleep and BLE modem sleep. Deep sleep is excluded because it disconnects BLE; GPIO wake for the ADC ladder requires board measurements. ADC scanning stays at 20 ms with a one-sample debounce. Work may use 160 MHz, idle may use 40 MHz, and BLE sleep uses the main crystal without assuming an external 32 kHz source.

Fully closing and reopening ES8311 caused a clipped startup transient on the tested board. Retaining codec bias/filter state avoids resetting the ADC and discarding the beginning of each recording. Residual analog current, net savings and wake latency require device measurements. LCD controller power and BLE connection parameters remain unchanged.

## Ownership and interfaces

The audio worker is the sole runtime owner of codec open/read/write/pause/suspend. `bsp_audio_pause(void)` stops both I2S channels and releases the audio sleep lock while retaining ES8311 bias/filter state. `bsp_audio_set_format()` resumes fresh DMA for the same format without resetting the codec; the first 16-bit mono read primes one partial I2S slot word (62.5 us at 16 kHz) before returning the complete requested frame. `bsp_audio_suspend()` fully closes the codec for teardown or a format change. Lifecycle calls are idempotent and serialized outside button callbacks. A partial pause retains its lock until all live channels stop; a failed resume supports retry. Retained analog power has a current cost that requires measurement.

`bsp_lvgl_set_paused(bool paused)` is called with the LVGL lock held. It pauses display refresh and animation timers and the port tick while off, and restores elapsed time on resume. The LVGL timer handler must remain enabled: disabling it in LVGL 9.5 returns a 1 ms delay and creates a polling loop. Battery updates cannot resume the display.

UI producers update a protected state snapshot and notify the worker; only that worker changes LVGL objects and backlight after initialization. Repeated identical host states do not reset the idle deadline. Brightness stays 65/38/18/0 percent, with 3-second dim and 20-second off deadlines; recording and processing retain the existing lit-screen behavior.

## Compatibility and failure policy

BLE V1, ADPCM framing, 16 kHz capture, boot sound priority, ADC key mapping, partitions and Recovery remain unchanged. Task notifications supplement state checks so an event arriving during audio pause cannot be lost. A failed audio start reports an error and returns to idle. Pausing LVGL must be reversible and checked for errors. All PM locks are balanced across repeated transitions.

USB standby polling (`main/usb_standby.c`) holds a no-light-sleep lock plus an 80 MHz APB-frequency floor while the native USB Serial/JTAG bus is active, with a five-second release grace for short host-side interruptions; an unplugged or long-suspended bus permits battery standby again. A failed audio pause retries or reports an error and returns to idle; it must not abort the device. Host suspend/replug behavior requires separate hardware acceptance.

## Validation and remaining acceptance

Run focused host tests and then `./tools/validate.sh` with ESP-IDF 5.5.3. Confirm the generated power-management and Bluetooth settings; retain the 3 MB application limit, protected partition offsets and Recovery hook.

Measure battery-side current for disconnected screen-off, connected screen-off, lit idle and recording. Exercise at least 20 sleep/wake/record/stop cycles, all three keys including long press, first-word capture, startup-sound interruption, disconnect during capture and reconnect while off. Verify that the USB console survives host serial-port probing and battery light sleep returns within the release grace after unplugging. Measure final current without a USB debugger. Report build, host tests and physical acceptance separately.
