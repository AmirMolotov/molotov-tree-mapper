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
  ])
    $(id).disabled =
      busy || (["all", "none", "generate"].includes(id) && !tree);
  document.querySelectorAll("#tree input,#tree button").forEach((el) => {
    if (busy) {
      el.dataset.wasDisabled = String(el.disabled);
      el.disabled = true;
    } else if (el.dataset.wasDisabled) {
      el.disabled = el.dataset.wasDisabled === "true";
      delete el.dataset.wasDisabled;
    }
  });
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
  for (const t of ["project", "excludes"]) {
    $("tab-" + t).setAttribute("aria-selected", String(t === name));
    $("tab-" + t).tabIndex = t === name ? 0 : -1;
    $(t + "-panel").hidden = t !== name;
  }
}
for (const name of ["project", "excludes"]) {
  $("tab-" + name).onclick = () => selectTab(name);
  $("tab-" + name).onkeydown = (e) => {
    if (!["ArrowLeft", "ArrowRight", "Home", "End"].includes(e.key)) return;
    e.preventDefault();
    const next =
      e.key === "Home"
        ? "project"
        : e.key === "End"
          ? "excludes"
          : name === "project"
            ? "excludes"
            : "project";
    selectTab(next);
    $("tab-" + next).focus();
  };
}
function render() {
  const host = $("tree");
  host.replaceChildren();
  let count = 0;
  function row(n, depth, parentOff) {
    const off = parentOff || excluded.has(n.path),
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
    check.disabled = parentOff;
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
    const text = await api().Generate([...excluded]);
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
    await api().CopyReport();
    status("Report copied.");
  });
$("save").onclick = () =>
  run(async () => {
    const p = await api().SaveReport();
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
attachDrop();
