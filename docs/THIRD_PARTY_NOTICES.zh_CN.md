<p align="right">
  <strong>简体中文</strong> · <a href="THIRD_PARTY_NOTICES.md">English</a>
</p>

# 第三方声明

本项目基于按 MIT License 分发的 [FoloToy/ai-passport](https://github.com/FoloToy/ai-passport)，其版权声明为 `Copyright (c) 2026 FoloToy`。完整上游声明保留在本仓库的 [`LICENSE`](../LICENSE) 中。

ESP-IDF Managed Component 的版本锁定在 [`dependencies.lock`](../dependencies.lock)。ESP-IDF 下载的各组件源码包中包含权威许可证和版权文件；再次分发这些组件时必须保留相应文件。

| 组件 | 锁定版本 | 组件发行包中的主要许可证 |
| --- | ---: | --- |
| Espressif ESP-IDF | 5.5.3 | Apache License 2.0；ESP-IDF 另行记录的第三方例外除外 |
| `espressif/button` | 4.2.0 | Apache License 2.0 |
| `espressif/cmake_utilities` | 1.1.1 | Apache License 2.0 |
| `espressif/esp_codec_dev` | 1.6.2 | Apache License 2.0，并包含设备相关声明 |
| `espressif/esp_lvgl_port` | 2.9.0 | Apache License 2.0 |
| `lvgl/lvgl` | 9.5.0 | MIT License，并包含额外的内置第三方声明 |

本表仅为阅读便利，不能替代各依赖随附的许可证文件。再次分发依赖源码或编译后的固件时，应根据实际锁定版本附带适用的许可证、版权声明、NOTICE 和组件内第三方文件。

Vokie 名称与图形标识不是第三方软件依赖，也不在 MIT 授权范围内；其独立再分发条款见 [`LICENSES/Vokie-Brand-Asset.txt`](../LICENSES/Vokie-Brand-Asset.txt)。

上游 Barlow Condensed Bold 子集保留为参考源码，版权归 2017 The Barlow Project Authors 所有，按 SIL Open Font License 1.1 分发，保留[完整字体许可](../LICENSES/Barlow-OFL.txt)。Go Voice 使用已启用的 LVGL Montserrat 20 字体，不再编译旧标题子集。见[资源目录](../assets/README.zh_CN.md#vokie-标题字体)。

Go Voice 导入 Vokie 固件提交 `ce26ea043251102155e54bc9503eb8133d5cafa8`，完整保留 FoloToy MIT 归属；其专有图形已从固件运行时资产移除。Go Voice PNG 是用户提供的自有图形，不是 Vokie 标志改色版本；本声明不授予独立的商标或图形权利。

参考开机 WAV 及其 16 kHz PCM 衍生文件从上述 Vokie 提交原样保留。上游称创作者允许其固件使用，但未提供独立音效许可证，该说明不能证明 Go Voice 再分发授权：发布包含音效的源码或固件前，须确认独立授权。见[素材来源](../assets/README.zh_CN.md#vokie-开机音)。
