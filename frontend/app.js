const $ = (id) => document.getElementById(id);
let tree = null,
  excluded = new Set(),
  collapsed = new Set(),
  busy = false;
const api = () => window.go?.main?.App;
function status(message) {
  $("status").textContent = message;
}
function invalidate() {
  $("preview").value = "";
  $("copy").disabled = $("save").disabled = true;
  status("Selection changed. Generate a new report.");
}
function controls() {
  document.querySelectorAll("#tree input,#tree button").forEach((el) => {
    if (busy) {
      el.dataset.wasDisabled = String(el.disabled);
      el.disabled = true;
    } else if (el.dataset.wasDisabled) {
      el.disabled = el.dataset.wasDisabled === "true";
      delete el.dataset.wasDisabled;
    }
  });
  for (const id of ["choose", "all", "none", "generate"])
    $(id).disabled = busy || (id !== "choose" && !tree);
}
async function run(fn) {
  if (busy) return;
  busy = true;
  controls();
  try {
    await fn();
  } catch (e) {
    status(String(e));
  } finally {
    busy = false;
    controls();
  }
}
function each(n, fn) {
  fn(n);
  for (const c of n.children || []) each(c, fn);
}
function render() {
  const host = $("tree");
  host.replaceChildren();
  let included = 0,
    total = 0;
  function row(n, depth, parentExcluded) {
    const off = parentExcluded || excluded.has(n.path);
    if (!n.dir) {
      total++;
      if (!off) included++;
    }
    const div = document.createElement("div");
    div.className = "row" + (off ? " excluded" : "");
    div.style.paddingLeft = `${depth * 18}px`;
    if (n.dir) {
      const toggle = document.createElement("button");
      toggle.textContent = collapsed.has(n.path) ? "▸" : "▾";
      toggle.setAttribute(
        "aria-label",
        `${collapsed.has(n.path) ? "Expand" : "Collapse"} ${n.name}`,
      );
      toggle.setAttribute("aria-expanded", !collapsed.has(n.path));
      toggle.onclick = () => {
        collapsed.has(n.path)
          ? collapsed.delete(n.path)
          : collapsed.add(n.path);
        render();
      };
      div.append(toggle);
    } else {
      const space = document.createElement("span");
      space.className = "file-space";
      div.append(space);
    }
    const label = document.createElement("label"),
      check = document.createElement("input");
    check.type = "checkbox";
    check.checked = !off;
    check.disabled = parentExcluded;
    check.setAttribute("aria-label", `Include ${n.name}`);
    if (n.dir && !off) {
      let partial = false;
      each(n, (c) => {
        if (excluded.has(c.path)) partial = true;
      });
      check.indeterminate = partial;
    }
    check.onchange = () => {
      if (check.checked) {
        each(n, (c) => excluded.delete(c.path));
      } else excluded.add(n.path);
      invalidate();
      render();
    };
    label.append(check);
    const name = document.createElement("span");
    name.textContent = n.name + (n.dir ? "/" : "");
    name.className = n.warning ? "warning" : n.dir ? "kind" : "";
    name.title = n.warning || n.path;
    label.append(name);
    div.append(label);
    host.append(div);
    if (!collapsed.has(n.path))
      for (const c of n.children || []) row(c, depth + 1, off);
  }
  row(tree, 0, false);
  // Counts include collapsed descendants as well as visible rows.
  included = 0;
  total = 0;
  function count(n, off) {
    off = off || excluded.has(n.path);
    if (!n.dir) {
      total++;
      if (!off) included++;
    }
    for (const c of n.children || []) count(c, off);
  }
  count(tree, false);
  $("count").textContent = `${included} / ${total} files`;
}
function loaded(n) {
  if (!n) return;
  tree = n;
  excluded.clear();
  collapsed.clear();
  each(n, (c) => {
    if (
      [
        ".git",
        "node_modules",
        ".env",
        ".env.local",
        ".env.production",
        "id_rsa",
        "id_ed25519",
      ].includes(c.name)
    )
      excluded.add(c.path);
  });
  invalidate();
  render();
  status(`Opened ${n.name}. Review selections, then generate.`);
}
$("choose").onclick = () =>
  run(async () => {
    if (!api())
      throw new Error("Open the desktop application to access local folders.");
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
    status("Reading selected files…");
    const text = await api().Generate([...excluded]);
    $("preview").value = text;
    $("copy").disabled = $("save").disabled = false;
    $("size").textContent =
      `${new TextEncoder().encode(text).length.toLocaleString()} bytes`;
    status("Report ready. Skipped files are labelled in the contents.");
  });
$("copy").onclick = () =>
  run(async () => {
    await api().CopyReport();
    status("Report copied to clipboard.");
  });
$("save").onclick = () =>
  run(async () => {
    const path = await api().SaveReport();
    status(path ? `Saved ${path}` : "Save cancelled.");
  });
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
  if (window.runtime?.OnFileDrop) {
    window.runtime.OnFileDrop(
      (x, y, paths) =>
        run(async () => {
          if (paths.length !== 1) throw new Error("Drop one folder at a time.");
          loaded(await api().LoadFolder(paths[0]));
        }),
      false,
    );
  } else setTimeout(attachDrop, 200);
}
attachDrop();
