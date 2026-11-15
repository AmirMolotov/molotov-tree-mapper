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


## File type groups (1.2)

After opening a folder, open **File types**. It scans filenames only and groups extensions into Images, Video, Audio, Documents, Code, Archives, Fonts, and Other. Toggle a category or an extension to include/exclude all matching files, including files in unexpanded folders. Matching is case-insensitive for extensions. These choices apply to the current project session, while the Excludes tab remains the persistent global name-rule layer. Include all clears manual and type selections but does not bypass saved name rules. Counts include files permitted by saved name rules, before individual tree-checkbox exclusions. Rescan after filesystem changes.

## Editable and encrypted exports (1.2)

The report preview is editable. Copy/save uses its current text, including your edits. To protect it, check **Encrypt copied/saved report**, choose **Generate key** or import/paste a valid key, then **Export key** separately. Encryption uses AES-256-GCM with a fresh random nonce for every export; keys come from Go's cryptographic random generator. Keys are 32 bytes encoded as base64, not passwords. Keys are not stored in SQLite or included in the report. Losing the key means the encrypted report cannot be recovered. Anyone holding the key can decrypt it: keep it separate from the report. The editable preview remains plaintext in application memory; encryption applies to copied/saved output, not the source files or clipboard history containing previous plaintext.

## Restore (1.2)

Open **Restore**, paste a report or choose **Open report**, and supply/import the matching key only if encrypted. **Decrypt for editing** puts plaintext in the input; **Validate report** previews the root, paths, counts and skipped-file count without writing anything. **Choose destination and restore** validates again, asks for a parent directory, and creates a new project folder inside it. An existing destination is refused. A failed restore cleans up its private staging directory and leaves existing projects untouched. Restored content is never automatically executed.

New exports include a compact versioned manifest between the title and tree. It records root/path identities, empty directories and skipped files, without duplicating file contents. Leave that metadata and the file headings/fences intact when editing file content. Structural path changes require a matching manifest; changing only the displayed tree does not rename restore destinations. The validation preview shows the actual destinations. The earlier 1.0/1.1 tree-and-fenced-content format is supported where unambiguous. Legacy text resembling skipped-file annotations is treated as skipped; older filename/warning ambiguities may need a fresh export.

Only included UTF-8 text files can be recreated. Binary/non-UTF-8, oversized or unreadable files previously marked skipped are not reconstructed; exclusions are not recovered. Timestamps, executable permissions, symlinks and other filesystem metadata are not preserved. New exports reject unsafe/nonportable names instead of producing an unrestorable report. Restore rejects traversal/absolute paths, Windows reserved names, case-conflicting duplicates, file/directory collisions, missing file bodies and malformed fences before writing. Plain reports are limited to 32 MiB; encrypted/base64 imports to 46 MiB.

No external cryptography library or custom cipher is used. See Go's [AES-GCM documentation](https://pkg.go.dev/crypto/cipher#NewGCM) and [cryptographic random source](https://pkg.go.dev/crypto/rand).
Editing or pasting text through the browser-based editors can normalize line endings. Unmodified generated reports and reports loaded from files retain their original bytes for export/decryption/restoration; after edits, the editor text is authoritative.

## Settings, LLM workflow, and local versions (1.3)

**Settings → Maximum generated output size** accepts 1–128 MiB (default 32), persisted in the app's SQLite settings. The report, optional prompt, and final copied/saved bytes must fit. Encrypted output includes base64 overhead, so a near-limit plaintext report may need a higher setting before encrypted export. The hard import ceiling is 128 MiB plaintext / 172 MiB encoded input. Source files above the hard ceiling remain skipped. This supersedes the fixed limits described for older versions above.

On the main Project page, check **Add project-conventions prompt for an LLM** and regenerate. The appended instruction asks the model to follow the project's organization, naming and existing modules and return complete changed/added file contents. The app strips its exact instruction block when parsing a report for restore. Model compliance is not guaranteed; validate and review returned changes before applying them.

Restore accepts the original report format, encrypted reports, or this LLM response (plain JSON or a single json fence, no surrounding prose):

```json
{
  "format": "molotov-llm-v1",
  "root": "my-project",
  "directories": ["src/empty"],
  "files": [
    {"path": "src/main.go", "content": "package main\n"}
  ]
}
```

`root` must match the existing project folder name when updating. Parent directories are inferred from paths. File content is mandatory; diffs, omitted bodies and prose are not guessed into files. Only listed files are changed/added; unmentioned existing files remain untouched. Explicit file deletions from an LLM reply are not supported. **Validate report** lists actual paths without writing; **Update existing project with history** asks for the target and confirms how many files will change. A new-folder restore also records an initial version.

History lives in **.molotov-history.sqlite** inside the project, automatically omitted from scans/exports and forbidden as an import target. It records full snapshots of managed UTF-8 text files. Before each update or revert, current managed file content is captured, including manual edits. The History tab can choose a project, select a version, revert one managed file, or revert managed files as a group. Reverts themselves produce new versions. Reverting to a state before a managed file existed removes that managed file after confirmation. Other files are not deleted, and empty directories are not removed. File permissions are preserved for replaced files; full filesystem metadata and binary history are not supported.

A persisted journal blocks subsequent changes after an interrupted update. **Recover interrupted update** restores its pre-update file state. If a file has since been edited externally to something other than either journaled state, recovery stops rather than overwriting that edit; preserve the external edit separately before resolving/retrying. This is file-by-file recovery, not an atomic filesystem transaction. Do not edit the same project in another application during an update/revert, or run two Molotov instances against the same history file. Snapshot sizes are bounded; history retention is not automatically pruned.

**History is local plaintext, even when an exported report is encrypted.** Keep the project/history on appropriate storage; the history database is not a substitute for an independent backup. Old restores performed before this feature have no retroactive history. Their first versioned update records the original content of affected files. Removing the database removes that local history. Updates to binary/non-UTF-8 files are rejected rather than discarding their original bytes.
