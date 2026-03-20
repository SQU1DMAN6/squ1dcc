# SQU1D++ Compiler Roadmap

## Goal
Turn SQU1DLang into a full compiler with standalone deployment, strong control-flow semantics, modern error handling, networking support, flexible function syntax, and better memory management.

## Task 1 (done): Create and track TODO
- [x] Create `TODO.md` listing planned tasks.

## Task 2 (do and done): Make build system work outside project tree + no deps
- [x] Add embedded mode fallback in `builder.BuildStandalone`.
  - If source binary exists, clone itself and append bytecode + footer.
  - Use `SQU1D_SOURCE_BINARY` override for tests.
- [x] Add `tryRunEmbedded()` in `main.go`.
  - Detect `embeddedMarker`, extract payload, run VM directly.
- [x] Keep legacy `go build` fallback for non-binary building.
- [x] Ensure `BuildStandalone` works from any working directory with only compiler binary.
- [x] Ensure generated executable runs standalone after relocation.

## Task 3: Full compiler mode
- [ ] Remove interpreter-only paths where possible (REPL vs compile mode separation).
- [ ] Confirm `main -B` path never drops to REPL.

## Task 4: Fix control flow and io.write behavior
- [ ] Add tests for `for`/`while` with `io.write` and true/false behavior.
- [ ] Fix boolean evaluation at bytecode/VM level.

## Task 5: New error handling package `<<`
- [ ] Implement runtime support for `<<` error pipe semantics.
- [ ] Document in language docs.

## Task 6: HTTP/HTTPS support
- [ ] Add builtin module `net.http` or `io.http`.
- [ ] Pattern match Gochi’s API.

## Task 7: New function definition syntax
- [ ] Add parsing for `var fn = >> () {...}` and `fn >> () {...}`.
- [ ] Wire compiler/VM behavior.

## Task 8: Memory management improvements
- [ ] Add object pooling / arena to VM.
- [ ] Evaluate performance compared to C++/Rust motivations.

## Validation
- [x] `go test ./...`
- [x] Manual cross-dir `squ1dcc -B -o /tmp/new.out /tmp/new.sqd`

