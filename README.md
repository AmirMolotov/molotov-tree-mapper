# Molotov Tree Mapper

A local Go desktop application that turns a folder into a readable directory tree followed by its file contents. Built with Wails and a small dependency-free HTML/CSS/JavaScript interface.

## Use

1. Drop one folder anywhere in the window, or choose **Open folder**.
2. Expand folders and uncheck any files or folders you want excluded. Parent exclusion excludes descendants. Folder selection includes its descendants again.
3. Choose **Generate report**, inspect the preview, then **Copy report** or **Save report** as Markdown/text.

`.git`, `node_modules`, common `.env` files and private-key filenames start excluded. This is a convenience, not secret detection: inspect selected content before sharing. Include all overrides those defaults. Source folders are never modified. No telemetry, uploads or remote processing.

## Windows build

Requires Go 1.23+, Microsoft Edge WebView2 runtime, and Wails CLI:

```powershell
go install github.com/wailsapp/wails/v2/cmd/wails@v2.10.2
wails build
```

Executable: `build/bin/molotov-tree-mapper.exe`. Development: `wails dev`. No Node build step or npm dependencies are required. Other operating systems require Wails' platform build dependencies; this release targets Windows.

```powershell
go test ./...
go vet ./...
```

## Report behavior and limits

- Stable ordering: folders first, then files, alphabetically within each group.
- UTF-8 text is included verbatim, with expandable Markdown fences for embedded backticks.
- Binary/non-UTF-8, inaccessible, changed/missing, and non-regular files are labelled as skipped; content is never silently truncated.
- Symbolic links are not traversed. Scans stop at 50,000 entries or 128 levels.
- Files over 2 MiB are labelled skipped. Reports over 32 MiB fail with an actionable error. Choose a smaller folder or exclude unnecessary content.
- A scan is a snapshot of names; contents are read when generating. Reopen the folder to discover new/deleted names. Avoid generating while another process changes the tree.
- Saving uses a temporary file beside the destination and a rename. Cancelled dialogs do nothing; write failures are reported.

## Structure

`tree.go`: scan, exclusion and report logic, independent of the UI. `app.go`: native dialogs, clipboard and synchronized application state. `main.go`: desktop host and embedded frontend. `frontend/`: accessible selection controls, preview, local styling. Tests exercise exclusions, fences, binary files, size limits, missing files and symlinks where OS permissions allow.

UI direction: a compact two-pane developer workspace, quiet neutral surfaces, a single accent, monospaced file/report content, keyboard focus and reduced-motion support. The taste skill's marketing-page hero/scroll scenes are intentionally inapplicable to this desktop workflow.
