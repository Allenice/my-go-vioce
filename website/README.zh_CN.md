简体中文 · [English](README.md)

# go-voice 网站

`go-voice` macOS 应用的介绍与下载网站：纯静态中文页面，部署在 Cloudflare Workers 静态资源上，域名 `go-voice.vfasky.com`。

- `public/`：站点文件。`img/app.webp` 是应用截图，`img/ai-passport.webp` 是 FoloToy AI Passport 硬件照片；图标由 `gui/resources/icon.png` 生成。
- `public/data/releases.json`：由同步脚本生成并被 Git 忽略，不要手改。
- `scripts/sync-releases.mjs`：读取 `vfasky/my-ai-audio` 已发布的 `go-voice-v*` GitHub release，获取 DMG 与 `SHA256SUMS.txt` 链接；更新说明取自 [gui/docs/CHANGELOG.zh_CN.md](../gui/docs/CHANGELOG.zh_CN.md) 中对应的 `## <版本>` 小节。网站只展示中文，GitHub 发布文档沿用原有语言规则。设置 `GITHUB_TOKEN` 可避免 API 限流。
- `cloudflare.config.ts`：Worker 名称、静态资源处理方式和自定义域名，由 `cf` CLI 部署。

## 发布流程

`go-voice` 在 GitHub 公开 release 后：

```sh
cd website && bun install && bun run deploy   # 等价于 sync + cf deploy
```

本地预览：先 `bun run sync`，再 `bun run dev`，打开 http://localhost:8787。

页面文案陈述的产品事实来自[应用指南](../gui/README.zh_CN.md)和[固件说明](../docs/go-voice-firmware.zh_CN.md)。这些事实变化时，在同一变更中更新网站。
