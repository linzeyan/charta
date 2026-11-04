//! The system's fonts, for backends that draw text themselves (the raster
//! behind Slint and PNG) or that need a face their toolkit does not bundle
//! (egui has no semibold). The library embeds no font (see spec): Inter when
//! installed, else the platform's sans, and per character the first
//! installed face that has the glyph, so CJK labels render instead of
//! showing boxes.

use std::sync::OnceLock;

use fontdb::{Database, Family, ID, Query};
use ttf_parser::Face;

use crate::base::text::{CAP_HEIGHT, Weight};

pub(crate) struct SystemFonts {
    pub db: Database,
    primary: [Option<ID>; 2],
    /// Cap height of the regular face, in ems: places Top and Middle
    /// baselines. egui places them from its own metrics.
    #[cfg_attr(not(feature = "png"), allow(dead_code))]
    pub cap: f64,
}

/// Families tried for text, in order: the Go port's list. Named rather than
/// left to fontdb's sans-serif, which on Linux is whichever fontconfig alias
/// it read last, often a font not installed.
const SANS: [Family; 10] = [
    Family::Name("Inter"),
    Family::Name("Helvetica Neue"),
    Family::Name("Helvetica"),
    Family::Name("Segoe UI"),
    Family::Name("Arial"),
    Family::Name("Noto Sans"),
    Family::Name("DejaVu Sans"),
    Family::Name("Liberation Sans"),
    Family::Name("Cantarell"),
    Family::SansSerif,
];

/// Families tried for characters the primary face lacks, before scanning
/// every installed face (whose order is arbitrary).
const FALLBACK: [&str; 8] = [
    "PingFang TC",
    "PingFang SC",
    "Hiragino Sans",
    "Noto Sans CJK TC",
    "Noto Sans CJK SC",
    "Microsoft JhengHei",
    "Microsoft YaHei",
    "Apple SD Gothic Neo",
];

pub(crate) fn system() -> &'static SystemFonts {
    static FONTS: OnceLock<SystemFonts> = OnceLock::new();
    FONTS.get_or_init(|| {
        let mut db = Database::new();
        db.load_system_fonts();
        let pick = |weight: u16| {
            db.query(&Query {
                families: &SANS,
                weight: fontdb::Weight(weight),
                ..Query::default()
            })
        };
        let primary = [pick(400), pick(600)];
        let cap = primary[0]
            .and_then(|id| {
                db.with_face_data(id, |data, i| {
                    let f = Face::parse(data, i).ok()?;
                    Some(f.capital_height()? as f64 / f.units_per_em() as f64)
                })
            })
            .flatten()
            .unwrap_or(CAP_HEIGHT);
        SystemFonts { db, primary, cap }
    })
}

impl SystemFonts {
    /// The face text of `weight` is set in; semibold falls back to regular.
    /// `None` when the system has no sans-serif at all.
    pub fn primary(&self, weight: Weight) -> Option<ID> {
        self.primary[(weight == Weight::Semibold) as usize].or(self.primary[0])
    }

    /// Faces to try for a character, in order: the primary face, the CJK
    /// families, then every installed face.
    pub fn candidates(&self, weight: Weight) -> impl Iterator<Item = ID> + '_ {
        let named = FALLBACK.iter().filter_map(|n| {
            self.db.query(&Query {
                families: &[Family::Name(n)],
                ..Query::default()
            })
        });
        self.primary(weight)
            .into_iter()
            .chain(named)
            .chain(self.db.faces().map(|f| f.id))
    }

    /// The first candidate face with a glyph for `c`.
    pub fn face_for(&self, c: char, weight: Weight) -> Option<ID> {
        self.candidates(weight).find(|&id| {
            self.db
                .with_face_data(id, |data, i| {
                    Face::parse(data, i).is_ok_and(|f| f.glyph_index(c).is_some())
                })
                .unwrap_or(false)
        })
    }
}
