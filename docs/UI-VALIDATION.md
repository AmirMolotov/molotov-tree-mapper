# UI validation

GATE: FAIL 23 (round 1 of 3).

The taste skill was applied as a design reference to a functioning desktop workspace. Its marketing-page gate is not passed: it expects a full-screen hero, navigation, photographic/mock visual sections, marketing heading scales, section-separated content edges, and a .wrap. The desktop uses adjacent tree/report panes and intentionally compact controls instead. No waivers were fabricated and the checker was not modified. Other flagged categories include button sizes, category wording, heading scale, visual presence and container edges. This is an open validation limitation, not a claimed gate pass.

Kind: software / developer utility. Actual design: dark neutral two-pane workspace, lime accent, system UI typography and monospaced source/report text; no photography, remote fonts, fabricated metrics, stock images, decorative scenes or scroll moments. The dealer suggested stone-blueprint and Host Grotesk; those were not applied. The desktop keeps locally available fonts and a dark workspace. Marketing-specific deals were not implemented.

Voice: short task-oriented instructions; actions name the object. Brand name supplied by the user, not coined by the assistant. No contact forms, external assets, stand-ins or canonical website. Keyboard controls, labels, focus indicators and reduced-motion styling are present.

The skill screenshot script rendered the initial workspace at 1440px and the result was visually inspected. The native Windows executable was launched and observed rendering its workspace, opening the folder dialog, and later showing a populated tree and generated report. Native drag-and-drop, clipboard and save have not each been independently exercised end to end by the assistant. Mobile/tablet screenshots and the separate performance script were not completed. Go tests and go vet passed; Windows production compilation succeeded. See README for file/report limits.

## Version 1.1

The project workspace was rendered and visually inspected at 1440px after adding Project/Excludes tabs. Rules-editor behavior, lazy folder expansion and the report loader are covered by dependency-free Node UI tests using a minimal DOM harness. Backend tests reopen the actual SQLite file, preserve an intentionally empty rule list, validate patterns, exercise lazy metadata versus full report traversal, and include text above the former 2 MiB limit. Native end-to-end interaction with the new tab has not been repeated. The earlier marketing-page gate failures remain open; no gate pass is claimed. Symlink tests were skipped on this Windows host because symlink creation privileges are unavailable.

## Version 1.2

Added editable exports, optional AES-256-GCM encryption with separate generated/imported/exported keys, a Restore tab with validation, and filename-extension groups. Six Node UI checks pass, including grouped extensions, edited encrypted export arguments, and restore input/key wiring. Go tests exercise encrypted round-trip restoration (including exact newlines, empty files/directories and embedded fences), wrong-key/tamper rejection, legacy reports, unsafe paths, collisions, skipped files, and case-insensitive type filters. The native Windows app was launched and the Restore tab/input were exercised; native dialog-based restore, clipboard encryption, and key-save flows were not completed end to end. The project view was screenshot-rendered at 1440px and inspected; encryption options were subsequently collapsed to preserve preview space. Previous marketing-page gate failures remain open, and no external security audit is claimed. Keys never enter SQLite. Type filters apply to the current project session.

## Version 1.3

Added persisted 1–128 MiB output settings; an optional project-conventions/LLM response prompt; strict LLM JSON parsing; and local SQLite version history with pre-change snapshots, file/full-managed-state reverts, and interrupted-update recovery. Go checks cover settings persistence and rejection, prompt-bearing report parsing, LLM response validation, managed file reverts, preservation of unrelated files and external edits, recovery journals, case conflicts and history exclusion. Ten Node UI checks cover the existing flows plus settings, prompt forwarding and selecting a version/file for revert. Native confirmation dialogs and journal recovery have not been fully exercised through the desktop UI. Earlier marketing-page gate failures remain open. History snapshots are plaintext and bounded; binary files, empty-directory removal, automatic pruning and LLM-requested deletion are not implemented.

## Version 1.3.1

Changed the LLM prompt and restore contract to the same tree-first, file-contents report format; removed the separate JSON reply parser. Language-tagged file fences and a final closing fence without a trailing newline are supported. File bodies must correspond to tree paths and order; restores/updates apply returned files in that order. Existing app exports with manifests remain supported. History derives Added/Modified/Removed/Existing changes from snapshots and displays per-version changes and file labels without a database migration.

Reproduced a size-settings UX regression with a failing Node test: editing the value without clicking Save left generation using the prior stored limit. The value now applies on change and is persisted again at generation; the active limit is displayed, and size errors include actual required size and budget. Go integration checks verify increasing the setting after opening a folder allows a previously-too-large report without reopening. Eleven UI tests, Go tests, go vet and Windows build pass. This reproduces the unsaved-setting path; the user's exact original error/size has not been supplied. Native UI dialogs have not been re-exercised end to end. The previous marketing-page gate remains unpassed.

## Version 1.3.2

Removed mandatory report/source/import/encryption/history byte ceilings and made the optional output cap default to 0 (unlimited). A transactional one-time migration disables previous saved caps; subsequent user-selected caps persist. Settings and the Project label show Unlimited, with an explicit Use unlimited output action. Regression tests cover generation that first fails under an opt-in cap then succeeds on the same open folder with unlimited, prompt/encryption/parse round trips, migration from a previous 64 MiB budget, preserving later opt-in caps above 128 MiB, and UI forwarding of 0. Twelve UI tests, Go tests and go vet pass; Windows production build succeeds. Native dialogs and large-memory stress tests were not completed. The implementation remains memory-based; physical RAM/disk and crypto/platform resource constraints still apply. The existing marketing-page gate remains unpassed.

## Workflow grouping and action hierarchy

Reorganized the existing desktop UI into grouped text-tab navigation: Workspace, Restore & history, and Preferences. Primary workflow actions use the accent; secondary actions use neutral controls; selection, key, and refresh utilities use quiet text actions. Import separates report input, review, and destinations. History separates the selected version, whole-version restore, an expandable single-file restore, and recovery. Optional report instructions/encryption share a disclosure above export controls. Updated stale folder-history guidance to match current behavior.

Visually inspected the actual initial workspace rendered at 1200x820, 760x580, and 390x844 (full-page captures are taller when scrolling is needed). Inspected Import and History interactively in local Chrome using computer use. The browser preview has no native Go bridge; its history fetch reports that limitation, so it does not validate native dialogs or filesystem writes. All original control IDs are retained. Seventeen Node UI tests pass, including section titles, panel visibility, and arrow-key navigation in visual order; Go tests, vet, and Windows production compilation pass. No fonts, UI dependencies, marketing visuals, or remote services were added. Native dialog workflows were not re-exercised. The marketing design gate remains a separate limitation for this desktop layout; no gate pass is claimed.

Design gate result: `GATE: OPEN 29 (rounds used up)`. No waivers were created. The run cleared the actual 320px overflow, preview field label, and heading-scale findings. Remaining findings concern the checker’s marketing-page layout requirements, which are retained here as open findings rather than converted into a marketing page:

1. hero-fill: workspace appears below a compact title rather than a full-screen hero.
2. flush: adjacent source/report panes share a screen.
3. nav: brand and grouped section tabs occupy separate rows.
4. text-only: report editor has no marketing visual.
5. cta: title area has no separate hero action.
6. copy-headline: Create a report does not name a product category being sold.
7. off-grid: desktop Workspace label is inset with its tab.
8. moment-none: no promotional animation moment.
9. hero-screen: grouped navigation is shorter than a screen.
10. hero-screen: title and workspace share the first screen.
11. section-height: status footer is shorter than a screen.
12. section-height: workspace is shorter than a screen.
13. section-air: title-to-workspace spacing is below the marketing spacing threshold.
14. one-section: multiple desktop UI regions share a screen.
15. one-section: main contains navigation directly.
16. one-section: main contains the workspace div and status footer directly.
17. one-heading: workspace has source and report headings.
18. one-heading: source and page headings share a desktop screen.
19. container-edges: header uses compact desktop margins.
20. container-edges: navigation/title/workspace use compact desktop margins.
21. container-edges: skip link is outside the marketing header shell.
22. container-edges: output-limit metadata is right aligned.
23. container-edges: headings align within their panes.
24. container-edges: report pane header is wider than half the workspace.
25. container-edges: editor/options/actions align to the report pane.
26. no-visual: source pane has no marketing image at mobile size.
27. off-grid: mobile Restore & history label is inset with its tabs.
28. off-grid: mobile Preferences label is inset with its tabs.
29. container: no marketing .wrap class.

Eleven nonblocking warnings remain, including copy-headings, copy-buttons, copy-actions, off-grid, and section-gaps. Product vocabulary, compact controls, adjacent working panes, and empty editors were retained for this existing desktop utility. No separate performance run was completed. No assumed business facts, new brand names, photographs, form endpoints, or external asset slots were introduced. Final screenshot at 320px was visually inspected after correcting overflow. Final executable includes the corrected layout.
