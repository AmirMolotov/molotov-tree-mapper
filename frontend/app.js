let originalPreview = null,
  originalRestore = null;
let excludedTypes = new Set(),
  typeCounts = [];
const $ = (id) => document.getElementById(id),
  api = () => window.go?.main?.App;
let tree = null,
  excluded = new Set(),
  collapsed = new Set(),
  busy = false;
function status(s) {
  $("status").textContent = s;
}
function invalidate() {
  originalPreview = null;
  $("preview").value = "";
  $("copy").disabled = $("save").disabled = true;
  status("Selection changed. Generate a new report.");
}
function controls() {
  $("loader").hidden = !busy;
  $("project-panel").setAttribute("aria-busy", String(busy));
  for (const id of [
    "choose",
    "all",
    "none",
    "generate",
    "save-excludes",
    "patterns",
    "include-prompt",
    "save-settings",
    "max-report",
    "choose-history",
    "refresh-history",
    "recover-history",
    "history-version",
    "history-file",
    "revert-file",
    "revert-version",
    "update-project",
    "scan-types",
    "new-key",
    "open-key",
    "save-key",
    "export-key",
    "encrypt-output",
    "open-report",
    "restore-input",
    "restore-key",
    "open-restore-key",
    "decrypt-report",
    "inspect-report",
    "restore-report",
    "preview",
  ])
    $(id).disabled =
      busy || (["all", "none", "generate"].includes(id) && !tree);
  document
    .querySelectorAll("#tree input,#tree button,#type-list input")
    .forEach((el) => {
      if (busy) {
        el.dataset.wasDisabled = String(el.disabled);
        el.disabled = true;
      } else if (el.dataset.wasDisabled) {
        el.disabled = el.dataset.wasDisabled === "true";
        delete el.dataset.wasDisabled;
      }
    });
  $("recover-history").disabled = busy || !historyData.pending;
}
async function run(fn, message = "Loading…") {
  if (busy) return;
  busy = true;
  $("loading-text").textContent = message;
  controls();
  try {
    await new Promise((r) =>
      requestAnimationFrame(() => requestAnimationFrame(r)),
    );
    await fn();
  } catch (e) {
    status(String(e));
  } finally {
    busy = false;
    controls();
  }
}
function selectTab(name) {
  for (const t of [
    "project",
    "excludes",
    "restore",
    "types",
    "settings",
    "history",
  ]) {
    $("tab-" + t).setAttribute("aria-selected", String(t === name));
    $("tab-" + t).tabIndex = t === name ? 0 : -1;
    $(t + "-panel").hidden = t !== name;
  }
}
for (const name of [
  "project",
  "excludes",
  "restore",
  "types",
  "settings",
  "history",
]) {
  $("tab-" + name).onclick = () => {
    selectTab(name);
    if (name === "history")
      run(async () => showHistory(await api().GetHistory()));
    if (name === "types" && tree && !typeCounts.length)
      $("scan-types").onclick();
  };
  $("tab-" + name).onkeydown = (e) => {
    if (!["ArrowLeft", "ArrowRight", "Home", "End"].includes(e.key)) return;
    e.preventDefault();
    const tabs = [
      "project",
      "excludes",
      "restore",
      "types",
      "settings",
      "history",
    ];
    const next =
      e.key === "Home"
        ? tabs[0]
        : e.key === "End"
          ? tabs[tabs.length - 1]
          : tabs[
              (tabs.indexOf(name) +
                (e.key === "ArrowLeft" ? tabs.length - 1 : 1)) %
                tabs.length
            ];
    selectTab(next);
    $("tab-" + next).focus();
  };
}
function render() {
  const host = $("tree");
  host.replaceChildren();
  let count = 0;
  function row(n, depth, parentOff) {
    const typeOff = !n.dir && excludedTypes.has(extension(n.name));
    const off = parentOff || excluded.has(n.path) || typeOff,
      div = document.createElement("div");
    div.className = "row" + (off ? " excluded" : "");
    div.style.paddingLeft = `${depth * 18}px`;
    if (n.dir) {
      const b = document.createElement("button"),
        closed = n.children === null || collapsed.has(n.path);
      b.textContent = closed ? "▸" : "▾";
      b.setAttribute(
        "aria-label",
        `${closed ? "Expand" : "Collapse"} ${n.name}`,
      );
      b.setAttribute("aria-expanded", String(!closed));
      b.onclick = () =>
        run(async () => {
          if (n.children === null)
            n.children = await api().LoadChildren(n.path);
          else if (!collapsed.has(n.path)) {
            collapsed.add(n.path);
            render();
            return;
          }
          collapsed.delete(n.path);
          render();
        }, `Loading ${n.name}…`);
      div.append(b);
    } else {
      const s = document.createElement("span");
      s.className = "file-space";
      div.append(s);
      count++;
    }
    const label = document.createElement("label"),
      check = document.createElement("input");
    check.type = "checkbox";
    check.checked = !off;
    check.disabled = parentOff || typeOff;
    check.setAttribute("aria-label", `Include ${n.name}`);
    check.indeterminate =
      n.dir &&
      !off &&
      [...excluded].some(
        (p) => p.startsWith(n.path ? n.path + "/" : "") && p !== n.path,
      );
    check.onchange = () => {
      if (check.checked) {
        for (const p of excluded)
          if (p === n.path || !n.path || p.startsWith(n.path + "/"))
            excluded.delete(p);
      } else excluded.add(n.path);
      invalidate();
      render();
    };
    const name = document.createElement("span");
    name.textContent = n.name + (n.dir ? "/" : "");
    name.title = n.warning || n.path;
    name.className = n.warning ? "warning" : n.dir ? "kind" : "";
    label.append(check, name);
    div.append(label);
    host.append(div);
    if (!collapsed.has(n.path))
      for (const c of n.children || []) row(c, depth + 1, off);
  }
  row(tree, 0, false);
  $("count").textContent = `${count} visible files`;
}
function loaded(n) {
  if (!n) return;
  tree = n;
  excludedTypes.clear();
  typeCounts = [];
  $("type-list").textContent =
    "Scan file types to list this project’s extensions.";
  excluded.clear();
  collapsed.clear();
  invalidate();
  render();
  selectTab("project");
  status(`Opened ${n.name}. Expand folders to load their contents.`);
}
$("choose").onclick = () =>
  run(async () => {
    if (!api())
      throw new Error("Open the desktop application to access folders.");
    loaded(await api().ChooseFolder());
  });
$("all").onclick = () => {
  excluded.clear();
  excludedTypes.clear();
  renderTypes();
  invalidate();
  render();
};
$("none").onclick = () => {
  excluded.add("");
  invalidate();
  render();
};
$("generate").onclick = () =>
  run(async () => {
    invalidate();
    await persistSize();
    const text = await api().Generate(
      [...excluded],
      [...excludedTypes],
      $("include-prompt").checked,
    );
    originalPreview = text;
    $("preview").value = text;
    $("copy").disabled = $("save").disabled = false;
    $("size").textContent =
      `${new TextEncoder().encode(text).length.toLocaleString()} bytes`;
    status(
      "Report ready. All included folders scanned; skipped files are labelled.",
    );
  }, "Generating report — scanning included folders and reading file contents…");
$("copy").onclick = () =>
  run(async () => {
    await api().CopyOutput(editorText("preview", originalPreview), outputKey());
    status("Report copied.");
  });
$("save").onclick = () =>
  run(async () => {
    const p = await api().SaveOutput(
      editorText("preview", originalPreview),
      outputKey(),
    );
    status(p ? `Saved ${p}` : "Save cancelled.");
  });
$("save-excludes").onclick = () =>
  run(async () => {
    await api().SaveExcludes($("patterns").value.split(/\r?\n/));
    $("patterns").value = (await api().GetExcludes()).join("\n");
    invalidate();
    if (tree) {
      tree = await api().LoadFolder(tree.root);
      excluded.clear();
      collapsed.clear();
      render();
    }
    status(
      "Excludes saved. Matching files and folders are omitted from every project.",
    );
  }, "Saving excludes and refreshing project…");
document.addEventListener("dragover", (e) => {
  e.preventDefault();
  document.body.classList.add("dropping");
});
document.addEventListener("dragleave", (e) => {
  if (!e.relatedTarget) document.body.classList.remove("dropping");
});
document.addEventListener("drop", (e) => {
  e.preventDefault();
  document.body.classList.remove("dropping");
});
function attachDrop() {
  if (!window.runtime?.OnFileDrop) {
    setTimeout(attachDrop, 200);
    return;
  }
  run(async () => {
    $("patterns").value = (await api().GetExcludes()).join("\n");
    $("max-report").value = (await api().GetSettings()).maxReportMiB;
    $("limit-label").textContent = `Limit: ${$("max-report").value} MiB`;
    status("Ready when you are.");
  }, "Loading saved excludes…");
  window.runtime.OnFileDrop(
    (x, y, paths) =>
      run(async () => {
        if (paths.length !== 1) throw new Error("Drop one folder at a time.");
        loaded(await api().LoadFolder(paths[0]));
      }),
    false,
  );
}
selectTab("project");
setTimeout(attachDrop, 0);

function outputKey() {
  if (!$("encrypt-output").checked) return "";
  const key = $("export-key").value.trim();
  if (!key)
    throw new Error("Generate or provide a key before encrypted export.");
  return key;
}
$("preview").oninput = () => {
  $("copy").disabled = $("save").disabled = !$("preview").value.trim();
};
$("new-key").onclick = () =>
  run(async () => {
    $("export-key").value = await api().NewKey();
    $("encrypt-output").checked = true;
    status(
      "Key generated. Export it separately before sharing the encrypted report.",
    );
  });
$("open-key").onclick = () =>
  run(async () => {
    const text = await api().OpenText();
    if (text) {
      $("export-key").value = text.trim();
      $("encrypt-output").checked = true;
    }
  });
$("save-key").onclick = () =>
  run(async () => {
    const p = await api().SaveKey($("export-key").value);
    status(
      p
        ? "Key exported. Keep it separate from the report."
        : "Key export cancelled.",
    );
  });
$("open-report").onclick = () =>
  run(async () => {
    const text = await api().OpenText();
    if (text) {
      originalRestore = text;
      $("restore-input").value = text;
      $("restore-summary").textContent =
        "Report loaded. Validate before restoring.";
    }
  });
$("open-restore-key").onclick = () =>
  run(async () => {
    const text = await api().OpenText();
    if (text) $("restore-key").value = text.trim();
  });
$("decrypt-report").onclick = () =>
  run(async () => {
    originalRestore = await api().DecryptReport(
      editorText("restore-input", originalRestore),
      $("restore-key").value,
    );
    $("restore-input").value = originalRestore;
    $("restore-summary").textContent =
      "Plaintext ready for editing. Validate after changes.";
  }, "Decrypting report…");
$("inspect-report").onclick = () =>
  run(async () => {
    const p = await api().InspectReport(
      editorText("restore-input", originalRestore),
      $("restore-key").value,
    );
    $("restore-summary").textContent =
      `Root: ${p.root}\n${(p.dirs || []).length} directories · ${(p.files || []).length - (p.skipped || []).length} files to restore · ${(p.skipped || []).length} skipped\n` +
      (p.dirs || [])
        .map((d) => d + "/")
        .concat((p.files || []).filter((f) => !(p.skipped || []).includes(f)))
        .join("\n");
    status("Report validated. No files have been written.");
  }, "Validating report…");
$("restore-report").onclick = () =>
  run(async () => {
    const p = await api().RestoreReport(
      editorText("restore-input", originalRestore),
      $("restore-key").value,
    );
    status(p ? `Project restored to ${p}` : "Restore cancelled.");
  }, "Validating and restoring files…");
for (const id of ["restore-input", "restore-key"])
  $(id).oninput = () => {
    $("restore-summary").textContent =
      "Input changed. Validate the report again.";
  };

function extension(name) {
  const i = name.lastIndexOf(".");
  return i < 0 ? "" : name.slice(i).toLowerCase();
}
const typeGroups = {
  Images: [
    ".png",
    ".jpg",
    ".jpeg",
    ".gif",
    ".webp",
    ".svg",
    ".bmp",
    ".tiff",
    ".tif",
    ".ico",
    ".avif",
    ".heic",
    ".raw",
    ".psd",
  ],
  Video: [
    ".mp4",
    ".mkv",
    ".mov",
    ".avi",
    ".webm",
    ".wmv",
    ".m4v",
    ".mpeg",
    ".mpg",
  ],
  Audio: [
    ".mp3",
    ".wav",
    ".flac",
    ".aac",
    ".ogg",
    ".m4a",
    ".opus",
    ".wma",
    ".aiff",
  ],
  Documents: [
    ".pdf",
    ".doc",
    ".docx",
    ".xls",
    ".xlsx",
    ".ppt",
    ".pptx",
    ".txt",
    ".md",
    ".rtf",
    ".csv",
    ".odt",
  ],
  Code: [
    ".go",
    ".js",
    ".ts",
    ".jsx",
    ".tsx",
    ".py",
    ".java",
    ".c",
    ".cpp",
    ".h",
    ".rs",
    ".html",
    ".css",
    ".scss",
    ".json",
    ".yaml",
    ".yml",
    ".xml",
    ".sh",
    ".sql",
    ".toml",
  ],
  Archives: [".zip", ".gz", ".tar", ".7z", ".rar", ".bz2", ".xz"],
  Fonts: [".woff", ".woff2", ".ttf", ".otf"],
};
function renderTypes() {
  const host = $("type-list");
  host.replaceChildren();
  if (!typeCounts.length) {
    host.textContent = "No files found, or types have not been scanned.";
    return;
  }
  const grouped = new Map();
  for (const type of typeCounts) {
    const group =
      Object.keys(typeGroups).find((k) =>
        typeGroups[k].includes(type.extension),
      ) || "Other";
    if (!grouped.has(group)) grouped.set(group, []);
    grouped.get(group).push(type);
  }
  for (const [group, types] of grouped) {
    const section = document.createElement("section");
    section.className = "type-group";
    const label = document.createElement("label"),
      check = document.createElement("input");
    check.type = "checkbox";
    const selected = types.filter(
      (t) => !excludedTypes.has(t.extension),
    ).length;
    check.checked = selected === types.length;
    check.indeterminate = selected > 0 && selected < types.length;
    const title = document.createElement("strong");
    title.textContent =
      group + " · " + types.reduce((n, t) => n + t.count, 0) + " files";
    label.append(check, title);
    check.onchange = () => {
      for (const t of types) {
        if (check.checked) excludedTypes.delete(t.extension);
        else excludedTypes.add(t.extension);
      }
      invalidate();
      renderTypes();
      if (tree) render();
    };
    section.append(label);
    for (const type of types) {
      const label = document.createElement("label"),
        box = document.createElement("input"),
        text = document.createElement("span");
      box.type = "checkbox";
      box.checked = !excludedTypes.has(type.extension);
      text.textContent =
        (type.extension || "(no extension)") + " · " + type.count;
      box.onchange = () => {
        if (box.checked) excludedTypes.delete(type.extension);
        else excludedTypes.add(type.extension);
        invalidate();
        renderTypes();
        if (tree) render();
      };
      label.append(box, text);
      section.append(label);
    }
    host.append(section);
  }
}
$("scan-types").onclick = () =>
  run(async () => {
    if (!tree) throw new Error("Choose a folder first.");
    typeCounts = await api().FileTypes();
    renderTypes();
    status("File types loaded. Unchecked types are excluded from the report.");
  }, "Scanning file names and grouping types…");

function editorText(id, original) {
  const value = $(id).value;
  return original !== null && value === original.replace(/\r\n?/g, "\n")
    ? original
    : value;
}

let historyData = { root: "", versions: [], pending: false };
function showHistory(h) {
  if (!h || !h.root) return;
  historyData = h;
  $("history-root").textContent = h.root;
  const versions = $("history-version");
  versions.replaceChildren();
  for (const v of h.versions || []) {
    const o = document.createElement("option");
    o.value = String(v.id);
    o.textContent = `#${v.id} · ${new Date(v.created).toLocaleString()} · ${v.label} · ${(v.changes || []).length} changed files`;
    versions.append(o);
  }
  versions.value = String(h.versions?.[0]?.id || "");
  renderHistoryFiles();
  $("history-status").textContent = h.pending
    ? "An interrupted update must be recovered before more changes."
    : `${h.versions?.length || 0} versions. Reverts save the current state before changing managed files.`;
  $("recover-history").disabled = !h.pending;
}
function renderHistoryFiles() {
  const select = $("history-file");
  select.replaceChildren();
  const v = (historyData.versions || []).find(
    (v) => String(v.id) === $("history-version").value,
  );
  for (const file of v?.files || []) {
    const o = document.createElement("option");
    o.value = file;
    const change = (v.changes || []).find((c) => c.path === file);
    o.textContent = (change ? `[${change.status}] ` : "[Unchanged] ") + file;
    select.append(o);
  }
  select.value = v?.files?.[0] || "";
  const changes = v?.changes || [];
  $("history-changes").textContent = changes.length
    ? changes.map((c) => `${c.status}: ${c.path}`).join("\n")
    : "No file changes in this version.";
}
$("history-version").onchange = renderHistoryFiles;
$("include-prompt").onchange = invalidate;
async function persistSize() {
  const n = Number($("max-report").value);
  if (!Number.isInteger(n) || n < 1 || n > 128)
    throw new Error("Choose a whole number from 1 to 128 MiB.");
  await api().SaveSettings({ maxReportMiB: n });
  $("limit-label").textContent = `Limit: ${n} MiB`;
  return n;
}
$("save-settings").onclick = () =>
  run(async () => {
    const n = await persistSize();
    status(`Maximum output size saved: ${n} MiB.`);
  }, "Saving settings…");
$("max-report").onchange = () =>
  run(async () => {
    const n = await persistSize();
    status(`Maximum output size applied: ${n} MiB.`);
  }, "Applying size limit…");
$("choose-history").onclick = () =>
  run(async () => showHistory(await api().ChooseHistory()));
$("refresh-history").onclick = () =>
  run(async () => showHistory(await api().GetHistory()));
$("recover-history").onclick = () =>
  run(async () => {
    showHistory(await api().RecoverHistory());
    status("Interrupted change recovered.");
  }, "Recovering interrupted change…");
$("update-project").onclick = () =>
  run(async () => {
    const h = await api().UpdateRestore(
      editorText("restore-input", originalRestore),
      $("restore-key").value,
    );
    if (h?.root) {
      showHistory(h);
      selectTab("history");
      status("Versioned project update complete.");
    } else status("Update cancelled.");
  }, "Validating and applying versioned update…");
async function revertSelected(single) {
  const id = Number($("history-version").value);
  if (!id) throw new Error("Select a history version first.");
  const file = single ? $("history-file").value : "";
  if (single && !file) throw new Error("Select a managed file first.");
  showHistory(await api().RevertHistory(id, file));
  status("History operation finished.");
}
$("revert-file").onclick = () =>
  run(() => revertSelected(true), "Saving current state and reverting file…");
$("revert-version").onclick = () =>
  run(
    () => revertSelected(false),
    "Saving current state and reverting managed files…",
  );
