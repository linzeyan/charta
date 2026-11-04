//! Text measurement for layout.
//!
//! Layout only needs advance widths, so the default measurer reads a table
//! generated from Inter (`assets/inter-widths.txt`) instead of embedding a
//! font. Native backends pass their own measurer, because the toolkit draws
//! with its own font and Inter widths would misalign its labels.

use std::collections::HashMap;
use std::sync::OnceLock;

/// Font weights charts use.
#[derive(Clone, Copy, Debug, Default, PartialEq, Eq)]
pub enum Weight {
    #[default]
    Regular,
    Semibold,
}

/// Measures the advance width of a single line of text, in pixels.
pub trait TextMeasure {
    fn width(&self, text: &str, size: f64, weight: Weight) -> f64;
}

/// Inter's cap height as a fraction of the font size, used to place text by
/// its visible top or middle rather than by font-specific ascent metrics.
pub const CAP_HEIGHT: f64 = 0.727;

/// Width of a character missing from the table and not full-width.
const FALLBACK_EM: f64 = 0.6;

/// The default measurer: Inter metrics for Latin text, 1em for CJK.
#[derive(Clone, Copy, Debug, Default)]
pub struct InterMetrics;

struct Table {
    upem: u64,
    widths: HashMap<char, (u64, u64)>,
}

fn table() -> &'static Table {
    static TABLE: OnceLock<Table> = OnceLock::new();
    TABLE.get_or_init(|| {
        let mut upem = 0;
        let mut widths = HashMap::new();
        for line in include_str!("../../assets/inter-widths.txt").lines() {
            let f: Vec<&str> = line.split_whitespace().collect();
            match f.as_slice() {
                [] => {}
                [c, ..] if c.starts_with('#') => {}
                ["upem", n] => upem = n.parse().expect("upem"),
                [cp, r, s] => {
                    let cp = u32::from_str_radix(cp, 16).expect("codepoint");
                    let ch = char::from_u32(cp).expect("valid codepoint");
                    widths.insert(
                        ch,
                        (r.parse().expect("regular"), s.parse().expect("semibold")),
                    );
                }
                _ => panic!("malformed inter-widths line: {line}"),
            }
        }
        assert!(upem > 0, "inter-widths.txt lacks upem");
        Table { upem, widths }
    })
}

/// East Asian wide characters, which every CJK font draws one em wide.
fn is_wide(c: char) -> bool {
    matches!(c as u32,
        0x1100..=0x115F
        | 0x2E80..=0xA4CF
        | 0xAC00..=0xD7A3
        | 0xF900..=0xFAFF
        | 0xFE30..=0xFE4F
        | 0xFF00..=0xFF60
        | 0xFFE0..=0xFFE6
        | 0x20000..=0x3FFFD)
}

impl TextMeasure for InterMetrics {
    fn width(&self, text: &str, size: f64, weight: Weight) -> f64 {
        let t = table();
        let (mut units, mut wide, mut other) = (0u64, 0u64, 0u64);
        for c in text.chars() {
            match t.widths.get(&c) {
                Some(&(r, s)) => units += if weight == Weight::Regular { r } else { s },
                None if is_wide(c) => wide += 1,
                None => other += 1,
            }
        }
        units as f64 * size / t.upem as f64 + wide as f64 * size + other as f64 * size * FALLBACK_EM
    }
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn cjk_is_one_em_per_character() {
        assert_eq!(InterMetrics.width("營收", 12.0, Weight::Regular), 24.0);
    }

    #[test]
    fn latin_uses_inter_advances() {
        // "n" is 1210 units at 2048 upem in Inter 4.1 Regular.
        let w = InterMetrics.width("n", 2048.0, Weight::Regular);
        assert_eq!(w, 1210.0);
        assert!(
            InterMetrics.width("Revenue", 12.0, Weight::Semibold)
                > InterMetrics.width("Revenue", 12.0, Weight::Regular)
        );
    }

    #[test]
    fn unknown_characters_still_take_space() {
        assert!(InterMetrics.width("→", 10.0, Weight::Regular) > 0.0);
    }
}
