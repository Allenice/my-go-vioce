// ST7789P3 sleep/wake. Screen-off is backlight 0 plus this sequence; the UI
// worker must not hold the LVGL lock across the sleep-out delay.
#include "bsp_display.h"

#include "esp_lcd_panel_io.h"
#include "esp_lcd_panel_ops.h"
#include "esp_log.h"
#include "freertos/FreeRTOS.h"
#include "freertos/task.h"

static const char *TAG = "bsp_disp";

// Datasheet sleep-in/out. DISPOFF blanks the panel first so SLPIN does not
// flash a last frame onto a still-lit backlight.
#define ST7789_SLPIN  0x10
#define ST7789_SLPOUT 0x11
#define ST7789_SLPIN_HOLD_MS 5
#define ST7789_SLPOUT_DELAY_MS 120

static bool s_sleeping;

esp_err_t bsp_display_set_sleep(bool sleep)
{
    esp_lcd_panel_handle_t panel = bsp_display_panel();
    esp_lcd_panel_io_handle_t io = bsp_display_io();
    if (!panel || !io) return ESP_ERR_INVALID_STATE;
    if (sleep == s_sleeping) return ESP_OK;

    if (sleep) {
        esp_err_t err = esp_lcd_panel_disp_on_off(panel, false);
        if (err != ESP_OK) {
            ESP_LOGE(TAG, "DISPOFF failed: %s", esp_err_to_name(err));
            return err;
        }
        err = esp_lcd_panel_io_tx_param(io, ST7789_SLPIN, NULL, 0);
        if (err != ESP_OK) {
            ESP_LOGE(TAG, "SLPIN failed: %s", esp_err_to_name(err));
            return err;
        }
        vTaskDelay(pdMS_TO_TICKS(ST7789_SLPIN_HOLD_MS));
        s_sleeping = true;
        return ESP_OK;
    }

    esp_err_t err = esp_lcd_panel_io_tx_param(io, ST7789_SLPOUT, NULL, 0);
    if (err != ESP_OK) {
        ESP_LOGE(TAG, "SLPOUT failed: %s", esp_err_to_name(err));
        return err;
    }
    vTaskDelay(pdMS_TO_TICKS(ST7789_SLPOUT_DELAY_MS));
    err = esp_lcd_panel_disp_on_off(panel, true);
    if (err != ESP_OK) {
        ESP_LOGE(TAG, "DISPON failed: %s", esp_err_to_name(err));
        return err;
    }
    s_sleeping = false;
    return ESP_OK;
}
