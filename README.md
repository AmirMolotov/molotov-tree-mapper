# Molotov Tree Mapper

A local Go desktop application that turns a folder into a readable directory tree followed by its file contents. Built with Wails and a small dependency-free HTML/CSS/JavaScript interface.

## Use

1. Drop one folder anywhere in the window, or choose **Open folder**.
2. Expand folders and uncheck any files or folders you want excluded. Parent exclusion excludes descendants. Folder selection includes its descendants again.
3. Choose **Generate report**, inspect the preview, then **Copy report** or **Save report** as Markdown/text.

The **Excludes** tab saves global file/folder name patterns in SQLite at `%APPDATA%/Molotov Tree Mapper/settings.db` on Windows (the OS user configuration directory elsewhere). Put one pattern per line and choose **Save excludes**. Rules apply before folder traversal in both the source tree and reports, including nested matches. Saving refreshes the open project. An empty list stays empty after restart.

Defaults: `.git`, `node_modules`, `.env`, `.env.*`, `id_rsa`, `id_ed25519`. Patterns use Go name-glob semantics: `*` matches any characters, `?` one character, `[abc]` a character set. Examples: `*.log`, `temp*`, `file?.txt`. Matching is case-sensitive on all platforms. These are name rules at any depth, not path globs or gitignore negation rules. Invalid patterns are rejected without changing saved settings. Include all affects tree checkboxes but does not override saved rules; edit Excludes to restore a hidden name.

This is a convenience, not secret detection: inspect selected content before sharing. Source folders are never modified. No telemetry, uploads or remote processing.

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
node --test frontend_test.cjs
```

## Report behavior and limits

- Stable ordering: folders first, then files, alphabetically within each group.
- UTF-8 text is included verbatim, with expandable Markdown fences for embedded backticks.
- Binary/non-UTF-8, inaccessible, changed/missing, and non-regular files are labelled as skipped; content is never silently truncated.
- Symbolic links are not traversed. Scans stop at 50,000 entries or 128 levels.
- Browsing loads metadata only for the root directory and for folders you expand. It never reads file contents. Counts describe visible files, not the entire unexpanded project. Each expanded directory is enumerated as one listing; extremely wide directories are not paginated.
- Generating walks every included folder, even if never expanded, then reads complete UTF-8 file contents. A loader stays visible and editing is disabled while this runs. Text files over 2 MiB are now included. Files over 32 MiB are labelled skipped; reports exceeding 32 MiB fail with an actionable error. Choose a smaller folder or exclude unnecessary content.
- Expanded folders cache their listing until reopening or saving rules. Report generation always performs a fresh walk, respecting saved patterns and manual exclusions. Avoid generating while another process changes the tree.
- Saving uses a temporary file beside the destination and a rename. Cancelled dialogs do nothing; write failures are reported.

## Structure

`excludes.go`: transactional SQLite settings and validated name patterns. `tree.go`: lazy listings, full report walks, exclusion and report logic, independent of the UI. `app.go`: native dialogs, clipboard and synchronized application state. `main.go`: desktop host and embedded frontend. `frontend/`: accessible selection controls, preview, local styling. Tests exercise exclusions, fences, binary files, size limits, missing files and symlinks where OS permissions allow.

UI direction: a compact two-pane developer workspace, quiet neutral surfaces, a single accent, monospaced file/report content, keyboard focus and reduced-motion support. The taste skill's marketing-page hero/scroll scenes are intentionally inapplicable to this desktop workflow.
