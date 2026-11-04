//! Color roles and the light/dark palette (`assets/palette.txt`).
//!
//! Scenes carry roles, not colors, so the HTML backend can emit CSS
//! variables (theme switches without re-rendering) and native backends can
//! resolve the same roles for the toolkit's light or dark mode.

use std::sync::OnceLock;

/// Number of categorical series slots. A ninth series is never a generated
/// hue; charts must fold extra series into "Other" instead.
pub const SERIES_SLOTS: u8 = 8;

/// A color role.
#[derive(Clone, Copy, Debug, PartialEq, Eq, Hash)]
pub enum Color {
    Surface,
    TextPrimary,
    TextSecondary,
    TextMuted,
    Grid,
    Axis,
    Border,
    /// Categorical slot, zero-based (slot 0 is the palette's `series-1`).
    Series(u8),
    /// The same slot as a ~10% wash, for area fills.
    Wash(u8),
    /// The same slot at ~35%, for flows that overlap (ribbons, links): light
    /// enough that crossings stay visible, strong enough to follow.
    Link(u8),
    /// Sequential class, zero-based, low to high (`SEQ_CLASSES` of them).
    Seq(u8),
    /// Qualitative range band of a bullet graph, zero-based (`BAND_CLASSES`).
    Band(u8),
}

/// Classes in the sequential ramp. Past ~7, adjacent classes blur.
pub const SEQ_CLASSES: u8 = 7;
/// Qualitative ranges a bullet graph can show.
pub const BAND_CLASSES: u8 = 3;

impl Color {
    /// The palette token, also used as the CSS variable name (`--ch-<token>`).
    pub fn token(self) -> String {
        match self {
            Color::Surface => "surface".into(),
            Color::TextPrimary => "text-primary".into(),
            Color::TextSecondary => "text-secondary".into(),
            Color::TextMuted => "text-muted".into(),
            Color::Grid => "grid".into(),
            Color::Axis => "axis".into(),
            Color::Border => "border".into(),
            Color::Series(i) => format!("series-{}", i + 1),
            Color::Wash(i) => format!("series-{}-wash", i + 1),
            Color::Link(i) => format!("series-{}-link", i + 1),
            Color::Seq(i) => format!("seq-{}", i + 1),
            Color::Band(i) => format!("band-{}", i + 1),
        }
    }
}

/// An sRGB color with straight (unpremultiplied) alpha.
#[derive(Clone, Copy, Debug, PartialEq, Eq)]
pub struct Rgba(pub u8, pub u8, pub u8, pub u8);

impl Rgba {
    /// CSS hex form; the alpha byte is omitted when opaque.
    pub fn hex(self) -> String {
        let Rgba(r, g, b, a) = self;
        if a == 255 {
            format!("#{r:02x}{g:02x}{b:02x}")
        } else {
            format!("#{r:02x}{g:02x}{b:02x}{a:02x}")
        }
    }
}

/// Which palette a rendered chart uses.
#[derive(Clone, Copy, Debug, Default, PartialEq, Eq)]
pub enum Theme {
    /// Follow the viewer: OS color scheme, overridden by a `data-theme`
    /// attribute on the page root. HTML only; native backends ask the toolkit.
    #[default]
    Auto,
    Light,
    Dark,
}

/// One palette row: token, light value, dark value.
pub struct Entry {
    pub token: &'static str,
    pub light: Rgba,
    pub dark: Rgba,
}

/// The palette rows in file order.
pub fn palette() -> &'static [Entry] {
    static PALETTE: OnceLock<Vec<Entry>> = OnceLock::new();
    PALETTE.get_or_init(|| {
        include_str!("../../assets/palette.txt")
            .lines()
            .filter(|l| !l.trim().is_empty() && !l.starts_with('#'))
            .map(
                |l| match l.split_whitespace().collect::<Vec<_>>().as_slice() {
                    [token, light, dark] => Entry {
                        token,
                        light: parse_hex(light),
                        dark: parse_hex(dark),
                    },
                    _ => panic!("malformed palette line: {l}"),
                },
            )
            .collect()
    })
}

fn parse_hex(s: &str) -> Rgba {
    let byte = |i: usize| u8::from_str_radix(&s[i..i + 2], 16).expect("palette hex");
    match s.len() {
        6 => Rgba(byte(0), byte(2), byte(4), 255),
        8 => Rgba(byte(0), byte(2), byte(4), byte(6)),
        _ => panic!("palette color must be 6 or 8 hex digits: {s}"),
    }
}

/// The concrete color of a role in light or dark mode.
pub fn resolve(color: Color, dark: bool) -> Rgba {
    let token = color.token();
    let e = palette()
        .iter()
        .find(|e| e.token == token)
        .unwrap_or_else(|| panic!("palette has no {token}"));
    if dark { e.dark } else { e.light }
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn every_role_resolves_in_both_modes() {
        let mut roles = vec![
            Color::Surface,
            Color::TextPrimary,
            Color::TextSecondary,
            Color::TextMuted,
            Color::Grid,
            Color::Axis,
            Color::Border,
        ];
        roles.extend((0..SERIES_SLOTS).map(Color::Series));
        roles.extend((0..SERIES_SLOTS).map(Color::Wash));
        roles.extend((0..SERIES_SLOTS).map(Color::Link));
        roles.extend((0..SEQ_CLASSES).map(Color::Seq));
        roles.extend((0..BAND_CLASSES).map(Color::Band));
        for c in roles {
            resolve(c, false);
            resolve(c, true);
        }
    }

    #[test]
    fn dark_mode_is_selected_not_flipped() {
        // Slot 1 is re-stepped for the dark surface, not inverted.
        assert_eq!(resolve(Color::Series(0), false).hex(), "#2a78d6");
        assert_eq!(resolve(Color::Series(0), true).hex(), "#3987e5");
        assert_eq!(resolve(Color::Border, true).hex(), "#ffffff1a");
    }
}
