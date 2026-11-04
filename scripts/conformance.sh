#!/bin/bash
# Holds the Rust and Go ports to identical output on what lives in the repo,
# so it runs in CI (the full fixture set in docs/conformance is local only;
# see `make gallery`):
#   1. dmath: both ports evaluate the same inputs; the result bits must match.
#   2. demos: every gallery demo's scene dump and HTML must match.
#
# Usage: make test-conformance   (from the repo root; needs cargo and go)
# Output: build/conformance/{dmath-*.txt,rust/,go/,*.log,*.diff}
# Plain bash 3.2: macOS /bin/bash and the Linux runners both run it.

cd "$(dirname "$0")/.." || exit 2
OUT=build/conformance
rm -rf "$OUT/rust" "$OUT/go"
mkdir -p "$OUT"

cargo test -q --lib dmath::tests::writes_results >"$OUT/01-dmath-rust.log" 2>&1
s1=$?
go test -count=1 -run TestWritesResults ./internal/dmath >"$OUT/02-dmath-go.log" 2>&1
s2=$?
diff "$OUT/dmath-rust.txt" "$OUT/dmath-go.txt" >"$OUT/dmath.diff" 2>&1
s3=$?
cargo run -q -p charta-demos --example dump -- "$OUT/rust" >"$OUT/04-demos-rust.log" 2>&1
s4=$?
go run ./internal/cmd/demos "$OUT/go" >"$OUT/05-demos-go.log" 2>&1
s5=$?
diff -r "$OUT/rust" "$OUT/go" >"$OUT/demos.diff" 2>&1
s6=$?

echo "step               status"
echo "dmath rust         $s1"
echo "dmath go           $s2"
echo "dmath bits differ  $s3  ($(wc -l <"$OUT/dmath-rust.txt" 2>/dev/null) inputs)"
echo "demos rust         $s4"
echo "demos go           $s5"
echo "demos differ       $s6  ($(ls "$OUT/rust" 2>/dev/null | wc -l) files)"
for f in dmath demos; do
  [ -s "$OUT/$f.diff" ] && { echo "--- $OUT/$f.diff (head)"; head -20 "$OUT/$f.diff"; }
done
[ $s1 -eq 0 ] && [ $s2 -eq 0 ] && [ $s3 -eq 0 ] && [ $s4 -eq 0 ] && [ $s5 -eq 0 ] && [ $s6 -eq 0 ]
