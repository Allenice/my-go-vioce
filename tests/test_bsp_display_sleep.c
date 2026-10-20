#include "bsp_display.h"
#include "esp_lcd_panel_io.h"
#include "esp_lcd_panel_ops.h"
#include "freertos/FreeRTOS.h"
#include "freertos/task.h"
#include <assert.h>
#include <stdio.h>

static int panel, io;
static bool fail_on_off, fail_cmd;
static int on_off_calls, cmd_calls, delays;
static bool last_on;
static int last_cmd;
static int delay_ms[4];

esp_lcd_panel_handle_t bsp_display_panel(void) { return (esp_lcd_panel_handle_t)&panel; }
esp_lcd_panel_io_handle_t bsp_display_io(void) { return (esp_lcd_panel_io_handle_t)&io; }

esp_err_t esp_lcd_panel_disp_on_off(esp_lcd_panel_handle_t p, bool on_off)
{
    assert(p == (esp_lcd_panel_handle_t)&panel);
    if (fail_on_off) return ESP_FAIL;
    last_on = on_off;
    ++on_off_calls;
    return ESP_OK;
}

esp_err_t esp_lcd_panel_io_tx_param(esp_lcd_panel_io_handle_t handle, int lcd_cmd,
                                    const void *param, size_t param_size)
{
    assert(handle == (esp_lcd_panel_io_handle_t)&io);
    assert(param == NULL && param_size == 0);
    if (fail_cmd) return ESP_FAIL;
    last_cmd = lcd_cmd;
    ++cmd_calls;
    return ESP_OK;
}

void vTaskDelay(TickType_t ticks)
{
    assert(delays < 4);
    delay_ms[delays++] = (int)ticks;
}

int main(void)
{
    assert(bsp_display_set_sleep(false) == ESP_OK);
    assert(on_off_calls == 0 && cmd_calls == 0);

    assert(bsp_display_set_sleep(true) == ESP_OK);
    assert(on_off_calls == 1 && !last_on);
    assert(cmd_calls == 1 && last_cmd == 0x10);
    assert(delays == 1 && delay_ms[0] == 5);
    assert(bsp_display_set_sleep(true) == ESP_OK);
    assert(on_off_calls == 1 && cmd_calls == 1);

    delays = 0;
    assert(bsp_display_set_sleep(false) == ESP_OK);
    assert(cmd_calls == 2 && last_cmd == 0x11);
    assert(delays == 1 && delay_ms[0] == 120);
    assert(on_off_calls == 2 && last_on);
    assert(bsp_display_set_sleep(false) == ESP_OK);
    assert(on_off_calls == 2 && cmd_calls == 2);

    fail_on_off = true;
    assert(bsp_display_set_sleep(true) == ESP_FAIL);
    fail_on_off = false;
    fail_cmd = true;
    assert(bsp_display_set_sleep(true) == ESP_FAIL);
    fail_cmd = false;
    delays = 0;
    assert(bsp_display_set_sleep(true) == ESP_OK);
    assert(last_cmd == 0x10 && delays == 1 && delay_ms[0] == 5);

    fail_cmd = true;
    assert(bsp_display_set_sleep(false) == ESP_FAIL);
    fail_cmd = false;
    fail_on_off = true;
    delays = 0;
    assert(bsp_display_set_sleep(false) == ESP_FAIL);
    assert(last_cmd == 0x11 && delays == 1 && delay_ms[0] == 120);
    fail_on_off = false;
    delays = 0;
    assert(bsp_display_set_sleep(false) == ESP_OK);
    assert(last_on && delays == 1 && delay_ms[0] == 120);

    puts("Display sleep lifecycle tests: PASS");
}
