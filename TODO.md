# SQU1D++ Compiler Roadmap

## Goal
Turn SQU1DLang into a full compiler with standalone deployment, strong control-flow semantics, modern error handling, networking support, flexible function syntax, and better memory management.

## Task 1: New error handling package `<<`
- [x] Implement runtime support for `<<` error pipe semantics.
- [ ] Document in language docs.

## Task 2: HTTP/HTTPS and web server support
- [x] Add builtin module `net.http` using SQX plugins.
  - [x] HTTP server with request routing
  - [x] User registration and login
  - [x] Session management
  - [x] Static file serving
- [ ] Add HTTPS and SQL database support using `sql.*` using SQX plugins.
- [x] Add support for HTML, CSS, and JS parsing, as well as web backend rendering and routing and middleware using `web.*` SQX plugins.
  - [x] HTML login/register forms
  - [x] CSS styling with gradient backgrounds
  - [x] JavaScript AJAX form handling
- [ ] Read through Gochi and Bun API and compare functionality.
- [ ] Compare functionality with other backends like Mux and Flask.
- [ ] Pattern match Gochi's API.
- [x] Ensure usage of syntax, SQX, and web backend development using SQU1DLang is foolproof.
  - [x] Comprehensive example in `examples/squ1dweb/`
  - [x] Full documentation and README

## Task 3: Run tests using new SQX plugins
- [x] Make a basic web server with register/login functions, HTML and CSS, as well as some JS frontend capabilities in HTML.
  - [x] Registration endpoint with validation
  - [x] Login with session tokens
  - [x] Protected dashboard
  - [x] HTML/CSS/JavaScript frontend
- [x] Use cURL to test functionality and rendering ability. Ensure rendering is correct. Allow SQU1DLang to be rendered in HTML as frontend.
  - [x] HTTP GET/POST requests via plugin
  - [x] HTML content rendering
  - [x] Static asset serving
- [x] Build and verify in `examples/`.
  - [x] Verified in `examples/squ1dweb/`
  - [x] All tests passing
