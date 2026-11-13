# charta — interactive chart library, native in both Rust and Go.
#
#   make test     run Rust and Go tests
#   make gallery  render fixtures with both ports, diff scenes, build the gallery
#   make check    everything CI gates on; run before push or tag
#
# Target names follow ~/git/dev_docs/project-conventions/SKILL.md.

SHELL   := /bin/bash
OUT     := build

# Builds skip LTO, here and in CI: nothing ships a release binary yet.
export CARGO_PROFILE_RELEASE_LTO           ?= false
export CARGO_PROFILE_RELEASE_CODEGEN_UNITS ?= 16

.DEFAULT_GOAL := help
.PHONY: help setup build wasm site test test-conformance test-web lint fmt check gallery run-egui run-slint run-gio run-fyne run-mygo clean

help: ## Show this help
	@grep -E '^[a-zA-Z0-9_-]+:.*?## ' $(MAKEFILE_LIST) \
		| awk 'BEGIN {FS = ":.*?## "}; {printf "  \033[36m%-12s\033[0m %s\n", $$1, $$2}'

setup: ## Install the pinned tools (mise.toml: wasm-bindgen, wasm-opt) and the wasm target
	mise install
	rustup target add wasm32-unknown-unknown

build: ## Dev build (Rust workspace and Go packages)
	cargo build --workspace --all-features
	go build ./...

# Go runs even when Rust fails, so one run reports both.
test: ## Rust and Go tests (native backends render headless: a GPU, or Mesa's software drivers)
	cargo test --workspace --all-features --no-fail-fast; s=$$?; go test ./... && exit $$s

test-conformance: ## Diff Rust and Go output on the in-repo demos and dmath bits
	$(SHELL) scripts/conformance.sh </dev/null

# The CLI must match the wasm-bindgen crate; mise.toml pins it. A page
# downloads this, so it is built for size: opt-level z and wasm-opt -Oz take
# the gzipped .wasm from 457 to 307 KB, and a 3000-point scatterplot still
# lays out in about 5 ms a wheel step (4 ms at opt-level 3).
wasm: ## Build charta-wasm and zoom.js into build/wasm (web zoom)
	CARGO_PROFILE_RELEASE_OPT_LEVEL=z cargo build -p charta-wasm --target wasm32-unknown-unknown --release
	mise exec -- wasm-bindgen --target web --out-dir $(OUT)/wasm \
		target/wasm32-unknown-unknown/release/charta_wasm.wasm
	mise exec -- wasm-opt -Oz $(OUT)/wasm/charta_wasm_bg.wasm -o $(OUT)/wasm/charta_wasm_bg.wasm
	cp -f wasm/zoom.js $(OUT)/wasm/

# The check proves the wasm build reads the page's specs as the Go port wrote
# them: else a visitor's first zoom would swap in another chart.
site: wasm ## Build the GitHub Pages gallery (every chart, zoom via wasm) into build/site
	rm -rf $(OUT)/site $(OUT)/site-specs
	go run ./internal/cmd/site -commit "$$(git rev-parse --short HEAD)" $(OUT)/site $(OUT)/site-specs
	cargo run -q -p charta-wasm --example site_specs -- $(OUT)/site-specs
	cp -f $(OUT)/wasm/zoom.js $(OUT)/wasm/charta_wasm.js $(OUT)/wasm/charta_wasm_bg.wasm $(OUT)/site/

test-web: wasm ## Zoom charts in headless Chrome through zoom.js
	$(SHELL) scripts/web-zoom.sh </dev/null

lint: ## Same lint and format checks as CI
	cargo fmt --all --check
	cargo clippy --workspace --all-targets --all-features -- -D warnings
	@test -z "$$(gofmt -l .)" || { gofmt -l .; exit 1; }
	go vet ./...

fmt: ## Format in place
	cargo fmt --all
	gofmt -w .

check: lint test test-conformance test-web site ## Everything CI gates on

# The fixtures live in docs/ (local only), so this is a pre-push check, not CI.
gallery: ## Diff Rust/Go scenes on docs/conformance; write docs/gallery/index.html
	/opt/homebrew/bin/bash docs/scripts/gallery.sh </dev/null

run-egui: ## Open the egui gallery
	cargo run -p charta-egui-gallery

run-slint: ## Open the Slint gallery (GPL-3.0 app: Slint's license)
	cargo run -p charta-slint-gallery

run-gio: ## Open the Gio gallery (DARK=1 for the dark palette)
	go run ./cmd/gio-gallery $(if $(DARK),-dark)

run-fyne: ## Open the Fyne gallery (follows the system light/dark setting)
	go run ./cmd/fyne-gallery

run-mygo: ## Open the MyGo gallery (follows the system light/dark setting; no cgo)
	go run ./cmd/mygo-gallery

clean: ## Remove build output
	rm -rf $(OUT) target
