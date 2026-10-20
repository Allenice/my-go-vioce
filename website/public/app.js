(() => {
  const fmt = (iso) =>
    new Date(iso).toLocaleDateString("zh-CN", { year: "numeric", month: "long", day: "numeric" });

  const el = (tag, cls, text) => {
    const n = document.createElement(tag);
    if (cls) n.className = cls;
    if (text != null) n.textContent = text;
    return n;
  };

  function renderLog(releases) {
    const log = document.getElementById("log");
    log.replaceChildren();
    releases.forEach((r, i) => {
      const card = el("article", "entry" + (i >= 5 ? " extra" : ""));
      const head = el("header");
      head.append(el("h3", null, "v" + r.version));
      const date = el("time", null, fmt(r.date));
      date.dateTime = r.date;
      head.append(date);
      card.append(head);

      const ul = el("ul");
      (r.notes.zh || []).forEach((t) => ul.append(el("li", null, t)));
      card.append(ul);

      const foot = el("footer");
      if (r.downloads.appleSilicon) {
        const download = el("a", "dl-link", "Apple 芯片 DMG");
        download.href = r.downloads.appleSilicon;
        foot.append(download);
      }
      const gh = el("a", "dl-link quiet", "GitHub");
      gh.href = r.url;
      gh.rel = "noopener";
      foot.append(gh);
      card.append(foot);
      log.append(card);
    });

    const more = document.getElementById("more");
    if (releases.length > 5) {
      more.hidden = false;
      more.addEventListener("click", () => {
        log.classList.add("all");
        more.hidden = true;
      });
    }
  }

  function applyLatest(r) {
    document.querySelectorAll("[data-version]").forEach((n) => (n.textContent = "v" + r.version));
    document.querySelectorAll("[data-dl]").forEach((a) => {
      const url = r.downloads[a.dataset.dl];
      if (url) a.href = url;
    });
  }

  fetch("/data/releases.json")
    .then((res) => {
      if (!res.ok) throw new Error("release data unavailable");
      return res.json();
    })
    .then((data) => {
      applyLatest(data.releases[0]);
      renderLog(data.releases);
    })
    .catch(() => {
      const link = el("a", "sub", "查看 GitHub 更新日志");
      link.href = "https://github.com/vfasky/my-ai-audio/releases";
      document.getElementById("log").replaceChildren(link);
    });
})();
