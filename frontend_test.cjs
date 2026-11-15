const { test } = require("node:test");
const assert = require("node:assert/strict");
const vm = require("node:vm");
const fs = require("node:fs");
test("settings validate and persist the output size", async () => {
  const { get, api } = setup();
  let saved;
  api.SaveSettings = async (s) => (saved = s);
  get("max-report").value = "8";
  await get("save-settings").onclick();
  assert.deepEqual(JSON.parse(JSON.stringify(saved)), { maxReportMiB: 8 });
  saved = null;
  get("max-report").value = "0";
  await get("save-settings").onclick();
  assert.equal(saved, null);
  assert.match(get("status").textContent, /1 to 128/);
});
test("optional project prompt flag reaches generation", async () => {
  const { get, context, api } = setup();
  vm.runInContext(
    "loaded({name:'project',path:'',dir:true,children:[]})",
    context,
  );
  let flag;
  api.Generate = async (a, b, prompt) => {
    flag = prompt;
    return "report";
  };
  get("include-prompt").checked = true;
  await get("generate").onclick();
  assert.equal(flag, true);
});
test("history file revert selects a version and one managed path", async () => {
  const { get, api } = setup();
  const h = {
    root: "X:/project",
    pending: false,
    versions: [
      {
        id: 2,
        label: "Import",
        created: "2026-11-15T00:00:00Z",
        files: ["a.txt"],
      },
    ],
  };
  api.ChooseHistory = async () => h;
  let args;
  api.RevertHistory = async (...v) => {
    args = v;
    return h;
  };
  await get("choose-history").onclick();
  await get("revert-file").onclick();
  assert.deepEqual(args, [2, "a.txt"]);
  assert.equal(get("recover-history").disabled, true);
});
test("unchanged editors retain original CRLF bytes", () => {
  const { get, context } = setup();
  get("preview").value = "a\nb\n";
  context.raw = "a\r\nb\r\n";
  assert.equal(
    vm.runInContext("editorText('preview',raw)", context),
    "a\r\nb\r\n",
  );
  get("preview").value = "edited";
  assert.equal(vm.runInContext("editorText('preview',raw)", context), "edited");
});
test("file type groups exclude every extension in a category", async () => {
  const { get, context, api } = setup();
  api.FileTypes = async () => [
    { extension: ".png", count: 2 },
    { extension: ".jpg", count: 1 },
    { extension: ".mp3", count: 4 },
  ];
  vm.runInContext(
    "loaded({name:'project',path:'',dir:true,children:[{name:'photo.png',path:'photo.png',dir:false,children:null}]})",
    context,
  );
  await get("scan-types").onclick();
  const group = get("type-list").children[0],
    check = group.children[0].children[0];
  check.checked = false;
  check.onchange();
  assert.equal(
    vm.runInContext(
      "excludedTypes.has('.png') && excludedTypes.has('.jpg') && !excludedTypes.has('.mp3')",
      context,
    ),
    true,
  );
  assert.equal(get("tree").children[1].children[1].children[0].disabled, true);
  let received;
  api.Generate = async (paths, types) => {
    received = Array.from(types);
    return "report";
  };
  await get("generate").onclick();
  assert.deepEqual(received, [".png", ".jpg"]);
});
test("export uses edited text and refuses encryption without a key", async () => {
  const { get, api } = setup();
  let args;
  api.CopyOutput = async (...values) => {
    args = values;
  };
  get("preview").value = "edited report";
  get("encrypt-output").checked = true;
  await get("copy").onclick();
  assert.equal(args, undefined);
  assert.match(get("status").textContent, /provide a key/);
  get("export-key").value = "test-key";
  await get("copy").onclick();
  assert.deepEqual(args, ["edited report", "test-key"]);
});
test("restore input and key are passed together", async () => {
  const { get, api } = setup();
  let args;
  api.RestoreReport = async (...values) => {
    args = values;
    return "X:/project";
  };
  get("restore-input").value = "encrypted";
  get("restore-key").value = "key";
  await get("restore-report").onclick();
  assert.deepEqual(args, ["encrypted", "key"]);
  assert.match(get("status").textContent, /restored/);
});
function setup() {
  const elements = new Map();
  const el = () => ({
    children: [],
    style: {},
    dataset: {},
    value: "",
    disabled: false,
    hidden: false,
    classList: { add() {}, remove() {} },
    setAttribute(k, v) {
      this[k] = v;
    },
    append(...v) {
      this.children.push(...v);
    },
    replaceChildren() {
      this.children = [];
    },
    focus() {},
  });
  const get = (id) => {
    if (!elements.has(id)) elements.set(id, el());
    return elements.get(id);
  };
  const calls = [];
  const api = {
    LoadChildren: async (p) => {
      calls.push(p);
      return [
        { name: "deep.txt", path: "src/deep.txt", dir: false, children: null },
      ];
    },
    Generate: async () => "full report",
    SaveExcludes: async (r) => calls.push(r),
    GetExcludes: async () => ["node_modules", "*.log"],
    LoadFolder: async () => ({
      name: "project",
      root: "X:/project",
      path: "",
      dir: true,
      children: [],
    }),
  };
  const context = vm.createContext({
    document: {
      getElementById: get,
      createElement: el,
      querySelectorAll: () => [],
      addEventListener() {},
    },
    window: { go: { main: { App: api } } },
    setTimeout() {},
    requestAnimationFrame: (fn) => fn(),
    TextEncoder,
    console,
  });
  vm.runInContext(fs.readFileSync("frontend/app.js", "utf8"), context);
  return { get, context, calls, api };
}
test("expanding a folder fetches only its children", async () => {
  const { get, context, calls } = setup();
  vm.runInContext(
    "loaded({name:'project',root:'X:/project',path:'',dir:true,children:[{name:'src',path:'src',dir:true,children:null}]})",
    context,
  );
  assert.equal(calls.length, 0);
  await get("tree").children[1].children[0].onclick();
  assert.deepEqual(calls, ["src"]);
  assert.equal(get("tree").children.length, 3);
});
test("generation shows loader until completion", async () => {
  const { get, context, api } = setup();
  vm.runInContext(
    "loaded({name:'project',path:'',dir:true,children:[]})",
    context,
  );
  let finish;
  api.Generate = () => new Promise((r) => (finish = r));
  const pending = get("generate").onclick();
  await new Promise((r) => setImmediate(r));
  assert.equal(get("loader").hidden, false);
  assert.equal(get("choose").disabled, true);
  finish("complete");
  await pending;
  assert.equal(get("loader").hidden, true);
  assert.equal(get("preview").value, "complete");
  assert.equal(get("copy").disabled, false);
});
test("saving rules persists the editor and refreshes current project", async () => {
  const { get, context, calls } = setup();
  vm.runInContext(
    "loaded({name:'project',root:'X:/project',path:'',dir:true,children:[]})",
    context,
  );
  get("patterns").value = "node_modules\n*.log";
  await get("save-excludes").onclick();
  assert.deepEqual(Array.from(calls[0]), ["node_modules", "*.log"]);
  assert.equal(get("patterns").value, "node_modules\n*.log");
  assert.equal(get("preview").value, "");
});
