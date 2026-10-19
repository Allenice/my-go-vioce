idf_component_register(
    SRCS "main.c" "vokie_ble.c" "ui_status.c" "boot_sound.c" "usb_standby.c"
    INCLUDE_DIRS "." "../assets/brand"
    REQUIRES bsp bt esp_event esp_driver_usb_serial_jtag esp_hw_support esp_netif esp_timer esp_wifi nvs_flash json esp_pm
)

target_add_binary_data(${COMPONENT_LIB}
    "../assets/music/vokie-boot-16k.pcm" BINARY)
