<p align="right">
  <strong>简体中文</strong> · <a href="README.md">English</a>
</p>

# 资源目录（Assets）

## Go Voice 标志与标题

- 来源：[brand/go-voice-gopher-source.png](brand/go-voice-gopher-source.png)，保留依据用户参考与要求生成的原始图稿。圆胖的 Gopher 略微歪头，保留灵动眼神、小门牙和收在胸前的小爪子，右下角配有麦克风。最终图稿使用透明底白色图案：原白色区域保持白色，原黑色区域变为透明，包括背景和面部镂空；抗锯齿边缘保留部分透明度。派生的 [brand/go-voice-logo.png](brand/go-voice-logo.png) 为 1024×1024 RGBA，保持透明底用于生成固件资源。桌面图标 `gui/resources/icon.png` 使用相同白色图稿叠加不透明黑色圆角底，仅底块外边距透明；未使用 Vokie 图形。
- 使用 `"$WAKU_BUN_EXECUTABLE" tools/generate-go-voice-logo.mjs` 生成全部标志资源（需要带 `Bun.Image` 的 bundled Bun 和 FFmpeg）。Bun.Image 将原图缩至 1024px；FFmpeg 叠合黑底、调整灰阶以去除淡纹理并保留抗锯齿边缘，再应用内缩 48px、半径 216px 的圆角底。随后叠加 [brand/go-voice-microphone.png](brand/go-voice-microphone.png)，保留吉祥物原样，再将 RGB 置白，并以原白色覆盖率计算透明度（`new_alpha = old_alpha * old_gray / 255`），移除黑色底块，将面部线条变为透明。麦克风的可编辑源文件为 [brand/go-voice-microphone.svg](brand/go-voice-microphone.svg)；修改后先在 macOS 运行 `swift tools/render-go-voice-logo.swift assets/brand/go-voice-microphone.svg assets/brand/go-voice-microphone.png` 重新生成透明 1024px PNG，再运行主生成脚本。Bun.Image 将完成的图标缩小至 64×64；FFmpeg 将所有透明区域叠合到现有屏幕背景 `#0B1018` 并输出 RGB565 小端 [go_voice_logo_asset.h](brand/go_voice_logo_asset.h)。临时目录在 `finally` 中清理；原始图稿保持原样，派生 PNG 与头文件重新生成。头文件记录 PNG 的 SHA-256。
- 单独生成桌面图标可运行 `"$WAKU_BUN_EXECUTABLE" tools/generate-go-voice-app-icon.mjs`，将透明白色图稿叠合到黑底，在 1024px 画布上应用内缩 48px、半径 216px 的圆角。完整标志生成脚本也会调用此步骤，避免重新生成固件资源时把桌面图标还原成透明底。仅生成 APP 图标的命令不会改写标志 PNG、固件头文件或固件二进制。
- `main/ui_status.c` 通过 `main/CMakeLists.txt` 的品牌目录路径引用头文件。像素常量为 8192 字节，与旧图相同，无运行时解码或缩放缓冲区；高分辨率源图不嵌入固件。
- 标题使用已启用的 LVGL Montserrat 20 字体，包含全部 `Go Voice` 字符。标题位置、宽度、对齐和裁切不变，不新增字体或堆分配。
- 按用户要求制作的品牌素材用于本项目，本说明不主张独立再分发授权。见[第三方声明](../docs/THIRD_PARTY_NOTICES.zh_CN.md)。

## Vokie 开机音

- 来源：Vokie 提交 `ce26ea043251102155e54bc9503eb8133d5cafa8` 中的参考 `v2-blip-octave-tail.wav`，保留为 [vokie-boot-v2-blip-octave-tail.wav](music/vokie-boot-v2-blip-octave-tail.wav)。上游称创作者允许其固件使用，但未提供独立音效许可证；发布含该音效的 Go Voice 源码或固件前，须确认独立授权。
- 原始格式：2 秒、44.1 kHz、有符号 16 位单声道 WAV。
- 固件素材：[vokie-boot-16k.pcm](music/vokie-boot-16k.pcm)，16 kHz、有符号 16 位小端单声道 PCM，共 64,000 字节；不做响度归一化或变速。
- 集成：`main/CMakeLists.txt` 仅将 PCM 嵌入闪存。`main/boot_sound.c` 使用 640 字节栈缓冲区，在已有音频任务中以 55/100 输出音量每次开机播放一次；收到录音请求时让位，补零排空播放队列、静音并丢弃开机期间的麦克风采样。保持录音格式及麦克风增益，播放失败仅记录日志，不阻止语音输入。

在仓库根目录重新生成：

```bash
ffmpeg -nostdin -i assets/music/vokie-boot-v2-blip-octave-tail.wav \
  -ar 16000 -ac 1 -c:a pcm_s16le -f s16le \
  assets/music/vokie-boot-16k.pcm
```

本目录集中存放可复用的资源（字库、图片、音乐等），按资源类型分子目录管理。每个资源放在其类型对应的子目录，并记录放置路径、命名方式、集成方式与来源/许可。二进制资源（字体、图片、音频）不属于纯 markdown 文档，请勿与文档混放。涉及版权/授权的资源需注明来源与许可。

## 字库（fonts）

可复用的字库文件与生成的字库源码放在 `fonts/`。

- 命名要能反映字族、字重、字级与格式。
- 记录来源、许可、字符范围、转换命令与目标放置路径。
- 添加字库前评估 Flash 与内部 RAM 影响；ESP32-C3 无 PSRAM。
- 不提交许可不允许分发的字库。

## 图片（images）

可复用的源图与生成的显示资产放在 `images/`。

- 使用描述性命名，并记录尺寸、像素格式、转换步骤与目标路径。
- 优先采用适合 240 × 320 RGB565 显示的格式，并纳入 Flash 与内部 RAM 考量。
- 许可允许时保留可编辑源文件，并记录来源与许可。
- 图片中不得包含设备二维码秘密、凭证或个人数据。

## 音乐与音效（music）

可复用的音乐与音效源码放在 `music/`。

- 记录来源、许可、采样率、位深、声道、转换命令与目标路径。
- 与当前 BSP 音频路径匹配时优先采用 16 kHz、16 位单声道 PCM。
- 嵌入音频前评估 Flash 与内部 RAM 成本；长录音应流式或分块。
- 无再分发许可不提交媒体文件。

## Vokie 标题字体

- 来源：[Barlow Condensed Bold](https://github.com/google/fonts/blob/60824dce48f7dd28fe7d65559f2da1f6e04e585b/ofl/barlowcondensed/BarlowCondensed-Bold.ttf)，固定到 Google Fonts 提交 `60824dce48f7dd28fe7d65559f2da1f6e04e585b`。
- 作者与许可：Copyright 2017 The Barlow Project Authors；SIL Open Font License 1.1，完整许可保留在 [Barlow-OFL.txt](../LICENSES/Barlow-OFL.txt)。
- 文件：[源 TTF](fonts/BarlowCondensed-Bold.ttf)、[生成的 LVGL 字体](fonts/vokie_title_barlow_condensed_bold_22.c) 与[声明](fonts/vokie_title_font.h)。
- 仅包含 `Vokie Power` 所用字符：空格、`P`、`V`、`e`、`i`、`k`、`o`、`r`、`w`。标题若需要其他字符，须重新生成子集。
- 集成：仅保留为上游参考，`main/CMakeLists.txt` 不再编译该子集。Go Voice 使用 LVGL Montserrat 20，TTF 及该生成子集均不嵌入 Go Voice 固件。

在仓库根目录使用 `lv_font_conv` 1.5.3 重新生成：

```bash
lv_font_conv --size 22 --bpp 4 --format lvgl \
  --font assets/fonts/BarlowCondensed-Bold.ttf --symbols 'Vokie Power' \
  --no-compress --no-prefilter --lv-include lvgl.h \
  --lv-font-name vokie_title_barlow_condensed_bold_22 \
  -o assets/fonts/vokie_title_barlow_condensed_bold_22.c
```
