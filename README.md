# Molotov Tree Mapper

A local Windows desktop app that turns a folder into a Markdown directory tree and its file contents. Select files, export a report, or restore a project from one. Built with Go and Wails; no uploads or remote processing.

## Create a report

1. Open **Create report** and choose **Open folder**, or drop a folder into the window.
2. Uncheck files and folders you want to exclude. Use **File types** for extension filters and **Exclusion rules** for saved name patterns such as `node_modules` or `*.log`.
3. Choose **Generate report**. Watch file counts and live data sizes, or choose **Cancel report** to stop.
4. Review or edit the preview, then **Copy** or **Save file**. **Report options** includes optional LLM instructions and encrypted export.

Output is unlimited by default. **Output limits** accepts `0` for unlimited or a positive cap in MiB. Large reports still require enough memory.

## Restore and version history

- **Import report:** open or paste a report, supply its key if encrypted, and **Validate report**. Choose **Restore to new folder** or **Update with history** for an existing project.
- **Version history:** choose a project and saved version, then restore the whole version or a single file. Updates and reverts save the current managed state first. Created folders are removed only when empty; untracked content is preserved.
- **Recover update:** recover the previous state after an interrupted change. Avoid editing the project or using another app instance during updates and reverts.

LLM replies must use the same directory-tree and complete-file-contents format as exported reports. Include file sections in tree order, without an outer code fence or copied manifest. Unmentioned files remain unchanged on update. Always validate before applying.

## Privacy and limits

- Review reports before sharing. Exclusion rules are not secret detection.
- Only readable UTF-8 text is included; binary files are labelled skipped. Symlinks are not followed. Scans are limited to 50,000 entries and 128 levels.
- Encryption uses AES-256-GCM. Export and keep the key separately; a lost key cannot be recovered.
- History is **plaintext**, even for encrypted exports, in `.molotov-history.sqlite` inside the project. It is excluded from reports and is not a backup.
- Older file-only histories cannot reconstruct empty folders that were never recorded. Restore does not preserve full filesystem metadata.

## Build on Windows

Requires Go 1.23+ and Microsoft Edge WebView2 runtime:

```powershell
go install github.com/wailsapp/wails/v2/cmd/wails@v2.10.2
wails build
```

Executable: `build/bin/molotov-tree-mapper.exe`. Use `wails dev` for development. No npm dependencies or frontend build step are needed. Use Wails to build distributable executables so Windows icon, manifest, and version resources are included.

```powershell
go test ./...
go vet ./...
node --test frontend_test.cjs
```

Current version: **1.3.3**. See [UI validation notes](docs/UI-VALIDATION.md) for verification details and known limitations.
