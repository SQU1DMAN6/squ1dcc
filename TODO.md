# SQU1D++ Compiler Roadmap

## Goal
Turn SQU1DLang into a full compiler with standalone deployment, strong control-flow semantics, modern error handling, networking support, flexible function syntax, and better memory management.

## Task 1: New error handling package `<<`
- [x] Implement runtime support for `<<` error pipe semantics.
- [ ] Document in language docs.

## Task 2: HTTP/HTTPS and web server support
- [ ] Add builtin module `net.http` using SQX plugins.
- [ ] Add HTTPS and SQL database support using `sql.*` using SQX plugins.
- [ ] Add support for HTML, CSS, and JS parsing, as well as web backend rendering and routing and middleware using `web.*` SQX plugins.
- [ ] Read through Gochi and Bun API and compare functionality.
- [ ] Compare functionality with other backends like Mux and Flask.
- [ ] Pattern match Gochi’s API.
- [ ] Ensure usage of syntax, SQX, and web backend development using SQU1DLang is foolproof.

## Task 3: Run tests using new SQX plugins
- [ ] Make a basic web server with register/login functions, HTML and CSS, as well as some JS frontend capabilities in HTML.
- [ ] Use cURL to test functionality and rendering ability. Ensure rendering is correct. Allow SQU1DLang to be rendered in HTML as frontend.
- [ ] Build and verify in `examples/`.
