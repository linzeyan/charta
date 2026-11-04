// Package base holds the building blocks every chart shares: scales, ticks,
// number formatting, text measurement and the color theme.
//
// It is a port of the Rust crate's base module and must return identical
// values. Two rules keep it that way:
//
//   - No library log10/pow in layout math: their last-ulp results differ
//     between Go and Rust and can flip a floor at exact powers of ten.
//   - Products feeding an addition are wrapped in float64(...). The Go spec
//     lets the compiler fuse x*y+z into one FMA instruction (it does on
//     arm64), which rounds differently from Rust's separate multiply and add;
//     an explicit conversion forbids the fusion.
package base
