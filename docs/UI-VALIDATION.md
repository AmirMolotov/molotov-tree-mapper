# UI validation

GATE: FAIL 23 (round 1 of 3).

The taste skill was applied as a design reference to a functioning desktop workspace. Its marketing-page gate is not passed: it expects a full-screen hero, navigation, photographic/mock visual sections, marketing heading scales, section-separated content edges, and a .wrap. The desktop uses adjacent tree/report panes and intentionally compact controls instead. No waivers were fabricated and the checker was not modified. Other flagged categories include button sizes, category wording, heading scale, visual presence and container edges. This is an open validation limitation, not a claimed gate pass.

Kind: software / developer utility. Actual design: dark neutral two-pane workspace, lime accent, system UI typography and monospaced source/report text; no photography, remote fonts, fabricated metrics, stock images, decorative scenes or scroll moments. The dealer suggested stone-blueprint and Host Grotesk; those were not applied. The desktop keeps locally available fonts and a dark workspace. Marketing-specific deals were not implemented.

Voice: short task-oriented instructions; actions name the object. Brand name supplied by the user, not coined by the assistant. No contact forms, external assets, stand-ins or canonical website. Keyboard controls, labels, focus indicators and reduced-motion styling are present.

The skill screenshot script rendered the initial workspace at 1440px and the result was visually inspected. The native Windows executable was launched and observed rendering its workspace, opening the folder dialog, and later showing a populated tree and generated report. Native drag-and-drop, clipboard and save have not each been independently exercised end to end by the assistant. Mobile/tablet screenshots and the separate performance script were not completed. Go tests and go vet passed; Windows production compilation succeeded. See README for file/report limits.

## Version 1.1

The project workspace was rendered and visually inspected at 1440px after adding Project/Excludes tabs. Rules-editor behavior, lazy folder expansion and the report loader are covered by dependency-free Node UI tests using a minimal DOM harness. Backend tests reopen the actual SQLite file, preserve an intentionally empty rule list, validate patterns, exercise lazy metadata versus full report traversal, and include text above the former 2 MiB limit. Native end-to-end interaction with the new tab has not been repeated. The earlier marketing-page gate failures remain open; no gate pass is claimed. Symlink tests were skipped on this Windows host because symlink creation privileges are unavailable.
