const { test } = require("node:test");
const assert = require("node:assert/strict");
const vm = require("node:vm");
const fs = require("node:fs");
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
