import { defineConfig } from "cf/config";

export default defineConfig({
  worker: {
    name: "go-voice-website",
    compatibilityDate: "2026-10-01",
    observability: { enabled: true },
    assets: { htmlHandling: "auto-trailing-slash", notFoundHandling: "404-page" },
    domains: ["go-voice.yzsaas.net"],
  },
});
