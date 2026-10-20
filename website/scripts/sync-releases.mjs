// Generates public/data/releases.json at deploy time so visitors never call the GitHub API.
// Downloads come from go-voice-v* releases; bilingual notes come from the "## <version>"
// sections of gui/docs/CHANGELOG.md and CHANGELOG.zh_CN.md.
import { readFile, writeFile, mkdir } from "node:fs/promises";
import { dirname, join } from "node:path";
import { fileURLToPath } from "node:url";

const root = join(dirname(fileURLToPath(import.meta.url)), "..");
const repo = process.env.GITHUB_REPO || "vfasky/my-ai-audio";
const TAG_PREFIX = "go-voice-v";
const headers = { Accept: "application/vnd.github+json", "User-Agent": "go-voice-website" };
if (process.env.GITHUB_TOKEN) headers.Authorization = `Bearer ${process.env.GITHUB_TOKEN}`;

const res = await fetch(`https://api.github.com/repos/${repo}/releases?per_page=100`, { headers });
if (!res.ok) throw new Error(`GitHub API ${res.status}: ${await res.text()}`);

// Keep list items only and strip link, bold and code markup; the page renders plain text.
const plain = (s) => s.replace(/\[([^\]]+)\]\([^)]+\)/g, "$1").replace(/\*\*|`/g, "");
function bullets(text) {
  return text
    .split("\n")
    .map((l) => l.trim())
    .filter((l) => /^[-*]\s+/.test(l))
    .map((l) => plain(l.replace(/^[-*]\s+/, "")));
}
async function changelog(name) {
  const text = (await readFile(join(root, "../gui/docs", name), "utf8")).replace(/\r\n/g, "\n");
  const sections = {};
  for (const part of text.split(/^## /m).slice(1)) {
    const [heading, ...body] = part.split("\n");
    sections[heading.trim()] = bullets(body.join("\n"));
  }
  return sections;
}
const notesZh = await changelog("CHANGELOG.zh_CN.md");
const notesEn = await changelog("CHANGELOG.md");

const releases = [];
for (const r of await res.json()) {
  if (r.draft || r.prerelease || !r.tag_name.startsWith(TAG_PREFIX)) continue;
  const version = r.tag_name.slice(TAG_PREFIX.length);
  const find = (re) => r.assets.find((a) => re.test(a.name))?.browser_download_url ?? null;
  const zh = notesZh[version]?.length ? notesZh[version] : bullets((r.body || "").replace(/\r\n/g, "\n"));
  const en = notesEn[version] ?? [];
  if (!en.length) console.warn(`warning: gui/docs/CHANGELOG.md has no ${version} section; the English page will show Chinese notes.`);
  releases.push({
    version,
    tag: r.tag_name,
    date: r.published_at,
    url: r.html_url,
    notes: { zh, en: en.length ? en : zh },
    downloads: {
      appleSilicon: find(/macOS-AppleSilicon\.dmg$/),
      checksums: find(/^SHA256SUMS\.txt$/),
    },
  });
}
releases.sort((a, b) => new Date(b.date) - new Date(a.date));
if (!releases.length) throw new Error("no published go-voice release found");

await mkdir(join(root, "public/data"), { recursive: true });
await writeFile(
  join(root, "public/data/releases.json"),
  JSON.stringify({ repo, generatedAt: new Date().toISOString(), releases }, null, 2) + "\n",
);
console.log(`Wrote ${releases.length} releases, latest ${releases[0].version}`);
