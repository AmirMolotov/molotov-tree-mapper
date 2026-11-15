const { test } = require("node:test");
const assert = require("node:assert/strict");
const vm = require("node:vm");
const fs = require("node:fs");
test("generation uses an edited size setting without a separate save click", async () => {
  const { get, context, api } = setup();
  vm.runInContext(
    "loaded({name:'project',path:'',dir:true,children:[]})",
    context,
  );
  let saved = 1;
  api.SaveSettings = async (s) => {
    saved = s.maxReportMiB;
  };
  api.Generate = async () => {
    if (saved < 2) throw new Error("Report exceeds configured size limit");
    return "large report";
  };
  get("max-report").value = "2";
  await get("generate").onclick();
  assert.equal(get("preview").value, "large report");
});
test("settings validate and persist the output size", async () => {
  const { get, api } = setup();
  let saved;
  api.SaveSettings = async (s) => (saved = s);
  get("max-report").value = "8";
  await get("save-settings").onclick();
  assert.deepEqual(JSON.parse(JSON.stringify(saved)), { maxReportMiB: 8 });
  saved = null;
  get("max-report").value = "-1";
  await get("save-settings").onclick();
  assert.equal(saved, null);
  assert.match(get("status").textContent, /0 for unlimited/);
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
        changes: [{ path: "a.txt", status: "Added" }],
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
  assert.match(get("history-changes").textContent, /Added: a.txt/);
  assert.match(get("history-file").children[0].textContent, /Added/);
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
  get("max-report").value = "32";
  const api = {
    SaveSettings: async () => {},
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
    setInterval: () => 1,
    clearInterval() {},
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
test("unlimited output is persisted and used without a cap", async () => {
  const { get, context, api } = setup();
  let saved;
  api.SaveSettings = async (s) => (saved = s);
  await get("unlimited-output").onclick();
  assert.equal(saved.maxReportMiB, 0);
  assert.match(get("limit-label").textContent, /Unlimited/);
  vm.runInContext(
    "loaded({name:'project',path:'',dir:true,children:[]})",
    context,
  );
  api.Generate = async () => {
    assert.equal(saved.maxReportMiB, 0);
    return "report";
  };
  await get("generate").onclick();
  assert.equal(get("preview").value, "report");
});

test("report progress shows counts and cancellation stays available", async () => {
  const { get, context, api } = setup();
  vm.runInContext("loaded({name:'project',path:'',dir:true,children:[]})", context);
  let finish, cancelled = false;
  api.Generate = () => new Promise((r) => { finish = r; });
  api.GetReportProgress = async () => ({phase:"Reading",processed:3,total:10,included:2,skipped:1,sourceBytes:42,outputBytes:100});
  api.CancelReport = async () => { cancelled = true; };
  const pending = get("generate").onclick();
  await new Promise((r) => setImmediate(r));
  await vm.runInContext("refreshReportProgress()", context);
  assert.match(get("report-progress").textContent, /3 of 10 files/);
  assert.equal(get("report-meter").value, 3);
  assert.equal(get("cancel-report").hidden, false);
  assert.equal(get("cancel-report").disabled, false);
  await get("cancel-report").onclick();
  assert.equal(cancelled, true);
  finish("complete");
  await pending;
  assert.equal(get("cancel-report").hidden, true);
});

test("history distinguishes populated version 2 from absent baseline files", () => {
 const {get,context} = setup();
 context.history = {root:"X:/project",pending:false,versions:[
  {id:2,label:"Restore update",files:["a.txt"],presentFiles:1,absent:[],changes:[{path:"a.txt",status:"Added"}]},
  {id:1,label:"Before Restore update",files:["a.txt"],presentFiles:0,absent:["a.txt"],changes:[]}
 ]};
 vm.runInContext("showHistory(history)",context);
 assert.match(get("history-version").children[0].textContent,/1 files present/);
 get("history-version").value="1";
 get("history-version").onchange();
 assert.match(get("history-file").children[0].textContent,/Absent/);
 assert.match(get("history-changes").textContent,/0 files present/);
});

test("reverting refreshes the open project and clears its stale report", async () => {
 const {get,context,api} = setup();
 vm.runInContext("loaded({name:'project',root:'X:/project',path:'',dir:true,children:[{name:'a.txt',path:'a.txt',dir:false}]})",context);
 get("preview").value = "stale report";
 get("history-version").value = "1";
 api.RevertHistory = async () => ({root:"X:/project",pending:false,versions:[{id:4,files:[],presentFiles:0,changes:[]}]});
 let refreshed;
 api.LoadFolder = async root => { refreshed=root; return {name:"project",root,path:"",dir:true,children:[]}; };
 await get("revert-version").onclick();
 assert.equal(refreshed,"X:/project");
 assert.equal(get("preview").value,"");
 assert.equal(vm.runInContext("tree.children.length",context),0);
});

test("generation displays live read and generated data sizes", async () => {
 const {get,context,api} = setup();
 vm.runInContext("loaded({name:'project',path:'',dir:true,children:[]})",context);
 let finish;
 api.Generate=()=>new Promise(r=>{finish=r;});
 api.GetReportProgress=async()=>({phase:"Reading",processed:0,total:1,included:0,skipped:0,sourceBytes:0,readBytes:1572864,outputBytes:1024});
 const pending=get("generate").onclick();
 await new Promise(r=>setImmediate(r));
 await vm.runInContext("refreshReportProgress()",context);
 assert.match(get("report-progress").textContent,/1.50 MiB read/);
 assert.match(get("report-progress").textContent,/1.0 KiB generated/);
 finish("report");
 await pending;
});

test("section navigation follows workflow groups and updates context", () => {
 const {get,context} = setup();
 assert.equal(get("page-title").textContent,"Create a report");
 get("tab-project").onkeydown({key:"ArrowRight",preventDefault(){}});
 assert.equal(get("page-title").textContent,"Import a report");
 assert.equal(get("restore-panel").hidden,false);
 assert.equal(get("project-panel").hidden,true);
 get("tab-restore").onkeydown({key:"ArrowRight",preventDefault(){}});
 assert.equal(get("page-title").textContent,"Version history");
 assert.equal(get("history-panel").hidden,false);
 assert.equal(get("tab-history").tabIndex,0);
 get("tab-history").onkeydown({key:"End",preventDefault(){}});
 assert.equal(get("page-title").textContent,"Output limits");
 get("tab-settings").onkeydown({key:"ArrowRight",preventDefault(){}});
 assert.equal(get("page-title").textContent,"Create a report");
});
