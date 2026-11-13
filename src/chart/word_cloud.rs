use super::frame::{self, text};
use super::{Chart, PAD, check_size, count, describe, finite};
use crate::base::format;
use crate::base::text::{TextMeasure, Weight};
use crate::base::theme::Color;
use crate::error::Error;
use crate::scene::{Anchor, Baseline, Hit, Rect, Scene};

/// Font sizes run from the tick size up to this, or a quarter of the plot's
/// height when that is smaller.
const MIN_FONT: f64 = 11.0;
const MAX_FONT: f64 = 48.0;
/// Air around each word's box.
const WORD_PAD: f64 = 2.0;
/// The spiral's vertical step; the horizontal one is scaled by the plot's
/// aspect so the cloud fills a wide plot instead of a circle in its middle.
const STEP: f64 = 4.0;
const MAX_WORDS: usize = 200;
/// When words do not fit, every size shrinks by this factor and the cloud
/// is laid out again, until all fit or the largest word is at MIN_FONT.
const SHRINK: f64 = 0.85;

/// A word and how much it counts.
#[derive(Clone, Debug, PartialEq)]
#[cfg_attr(feature = "serde", derive(serde::Serialize, serde::Deserialize))]
pub struct Word {
    pub text: String,
    pub weight: f64,
}

/// Words sized by weight, packed around the center: heaviest first, each at
/// the first free spot on a spiral out from the middle.
///
/// # Example
///
/// ```
/// use charta::base::text::InterMetrics;
/// use charta::{Chart, WordCloudChart};
///
/// let chart = WordCloudChart::new()
///     .title("Words in chart reviews")
///     .word("data", 90.0)
///     .word("chart", 72.0)
///     .word("color", 60.0)
///     .word("axis", 50.0)
///     .word("legend", 44.0);
/// let scene = chart.scene(640.0, 400.0, &InterMetrics).unwrap();
/// // The scene's label is what a screen reader announces.
/// assert_eq!(scene.label, "Words in chart reviews. Word cloud of 5 words.");
/// ```
#[derive(Clone, Debug, Default, PartialEq)]
#[cfg_attr(feature = "serde", derive(serde::Serialize, serde::Deserialize))]
pub struct WordCloudChart {
    #[cfg_attr(
        feature = "serde",
        serde(default, skip_serializing_if = "Option::is_none")
    )]
    pub title: Option<String>,
    pub words: Vec<Word>,
}

impl WordCloudChart {
    pub fn new() -> Self {
        Self::default()
    }

    pub fn title(mut self, title: impl Into<String>) -> Self {
        self.title = Some(title.into());
        self
    }

    pub fn word(mut self, text: impl Into<String>, weight: f64) -> Self {
        self.words.push(Word {
            text: text.into(),
            weight,
        });
        self
    }
}

fn overlaps(a: &Rect, b: &Rect) -> bool {
    a.x < b.x + b.w && b.x < a.x + a.w && a.y < b.y + b.h && b.y < a.y + a.h
}

/// One word placed: its index, font size and box.
type Placed = (usize, f64, Rect);

/// Places the words in `order` with the largest at `top` px, each at the
/// first free spot on the spiral; the words that found none are counted.
fn place(
    words: &[Word],
    order: &[usize],
    top: f64,
    plot: Rect,
    measure: &dyn TextMeasure,
) -> (Vec<Placed>, usize) {
    let (cx, cy) = (plot.x + plot.w / 2.0, plot.y + plot.h / 2.0);
    let heaviest = words.iter().map(|w| w.weight).fold(0.0, f64::max);
    // A square spiral of legs 1, 1, 2, 2, 3, 3… steps (right, down,
    // left, up): integer step counts, so both ports visit exactly the
    // same spots, with no trigonometry to differ in the last bit.
    let (dx, dy) = (STEP * plot.w / plot.h, STEP);
    // A leg of n steps reaches about n/2 steps from the center, and the
    // plot is h/dy steps across either way (dx scales with the aspect).
    let legs = (plot.h / dy).ceil() as i64 + 2;
    let mut placed: Vec<Placed> = Vec::new();
    let mut dropped = 0;
    for &i in order {
        let w = &words[i];
        // Area tracks weight: the side of a word's box grows with the
        // square root.
        let size = (top * (w.weight / heaviest).sqrt()).max(MIN_FONT);
        let (bw, bh) = (
            measure.width(&w.text, size, Weight::Semibold) + 2.0 * WORD_PAD,
            size + 2.0 * WORD_PAD,
        );
        let fits = |sx: i64, sy: i64| {
            let r = Rect {
                x: cx + sx as f64 * dx - bw / 2.0,
                y: cy + sy as f64 * dy - bh / 2.0,
                w: bw,
                h: bh,
            };
            let inside = r.x >= plot.x
                && r.y >= plot.y
                && r.x + r.w <= plot.x + plot.w
                && r.y + r.h <= plot.y + plot.h;
            (inside && !placed.iter().any(|p| overlaps(&p.2, &r))).then_some(r)
        };
        let mut spot = fits(0, 0);
        let (mut sx, mut sy) = (0i64, 0i64);
        'spiral: for leg in 1..=2 * legs {
            if spot.is_some() {
                break;
            }
            let (ux, uy) = [(1, 0), (0, 1), (-1, 0), (0, -1)][((leg - 1) % 4) as usize];
            for _ in 0..(leg + 1) / 2 {
                sx += ux;
                sy += uy;
                spot = fits(sx, sy);
                if spot.is_some() {
                    break 'spiral;
                }
            }
        }
        match spot {
            Some(r) => placed.push((i, size, r)),
            // Smaller words come later and may still fit.
            None => dropped += 1,
        }
    }
    (placed, dropped)
}

impl Chart for WordCloudChart {
    fn scene(&self, width: f64, height: f64, measure: &dyn TextMeasure) -> Result<Scene, Error> {
        check_size(width, height)?;
        if self.words.len() > MAX_WORDS {
            return Err(Error::InvalidData(format!(
                "{} words, at most {MAX_WORDS}: a cloud of more is noise",
                self.words.len()
            )));
        }
        for (i, w) in self.words.iter().enumerate() {
            finite(&format!("{:?}'s weight", w.text), w.weight)?;
            if w.weight <= 0.0 || w.text.trim().is_empty() {
                return Err(Error::InvalidData(format!(
                    "word {i} needs text and a positive weight"
                )));
            }
            if self.words[..i].iter().any(|o| o.text == w.text) {
                return Err(Error::InvalidData(format!("{:?} is listed twice", w.text)));
            }
        }
        let mut marks = Vec::new();
        let top = frame::title(&mut marks, self.title.as_deref());
        let plot = Rect {
            x: PAD,
            y: top,
            w: (width - 2.0 * PAD).max(1.0),
            h: (height - PAD - top).max(1.0),
        };
        let max_font = MAX_FONT.min(plot.h / 4.0).max(MIN_FONT);
        let mut order: Vec<usize> = (0..self.words.len()).collect();
        order.sort_by(|&a, &b| self.words[b].weight.total_cmp(&self.words[a].weight));

        // Shrink the whole cloud rather than lose words: sizes keep their
        // ratios down to the MIN_FONT floor. Of the tries, the one that
        // drops fewest wins, the larger on a tie; what still does not fit is
        // left to the count in the accessible name.
        let mut largest = max_font;
        let (mut placed, mut dropped) = place(&self.words, &order, largest, plot, measure);
        while dropped > 0 && largest > MIN_FONT {
            largest *= SHRINK;
            let (p, d) = place(&self.words, &order, largest, plot, measure);
            if d < dropped {
                (placed, dropped) = (p, d);
            }
        }
        for (i, size, r) in placed {
            let w = &self.words[i];
            marks.push(
                text(
                    r.x + r.w / 2.0,
                    r.y + r.h / 2.0,
                    &w.text,
                    size,
                    Weight::Semibold,
                    Anchor::Middle,
                    Baseline::Middle,
                    Color::Series(0),
                )
                .hit(Hit::new(i.to_string(), w.text.as_str(), format::value(w.weight)).area(r)),
            );
        }
        let shown = self.words.len() - dropped;
        let mut what = format!("Word cloud of {}", count(shown, "word", "words"));
        if dropped > 0 {
            what += &format!("; {dropped} more did not fit");
        }
        what.push('.');
        Ok(Scene {
            width,
            height,
            label: describe(self.title.as_deref(), &what),
            marks,
        })
    }
}

#[cfg(test)]
mod tests {
    use super::*;
    use crate::base::text::InterMetrics;
    use crate::scene::Shape;

    fn boxes(s: &Scene) -> Vec<(String, f64, Rect)> {
        s.marks
            .iter()
            .filter_map(|m| match (&m.shape, &m.hit) {
                (Shape::Text { text, size, .. }, Some(h)) => {
                    Some((text.clone(), *size, h.area.unwrap()))
                }
                _ => None,
            })
            .collect()
    }

    #[test]
    fn words_never_overlap_and_the_heaviest_takes_the_middle() {
        let mut c = WordCloudChart::new();
        for (i, w) in ["data", "chart", "axis", "scale", "color", "mark", "legend"]
            .iter()
            .enumerate()
        {
            c = c.word(*w, 70.0 - 10.0 * i as f64);
        }
        let s = c.scene(640.0, 400.0, &InterMetrics).unwrap();
        let b = boxes(&s);
        assert_eq!(b.len(), 7);
        for (i, x) in b.iter().enumerate() {
            for y in &b[i + 1..] {
                assert!(!overlaps(&x.2, &y.2), "{} and {} overlap", x.0, y.0);
            }
        }
        let first = &b[0];
        assert_eq!(first.0, "data");
        let (mx, my) = (first.2.x + first.2.w / 2.0, first.2.y + first.2.h / 2.0);
        assert!((mx - 320.0).abs() < 1e-9, "{mx}");
        assert!(my > 100.0 && my < 300.0);
        assert!(
            b.windows(2).all(|w| w[0].1 >= w[1].1),
            "sizes follow weight"
        );
    }

    #[test]
    fn a_crowded_cloud_shrinks_to_keep_every_word() {
        // At full size these words overflow the plot; the cloud is set
        // smaller instead, so no word is lost and the weights still rank.
        let mut c = WordCloudChart::new();
        for i in 0..40 {
            c = c.word(format!("term{i}"), 100.0 - i as f64 * 2.0);
        }
        let (w, h) = (320.0, 200.0);
        let s = c.scene(w, h, &InterMetrics).unwrap();
        let b = boxes(&s);
        assert_eq!(b.len(), 40, "{}", s.label);
        assert!(!s.label.contains("did not fit"), "{}", s.label);
        // Alone, the heaviest word has room for its full size.
        let alone = WordCloudChart::new().word("term0", 100.0);
        let full = boxes(&alone.scene(w, h, &InterMetrics).unwrap())[0].1;
        assert!(b[0].1 < full, "the largest word kept its {full} px");
        assert!(
            b.windows(2).all(|w| w[0].1 >= w[1].1),
            "sizes follow weight"
        );
        for (i, x) in b.iter().enumerate() {
            for y in &b[i + 1..] {
                assert!(!overlaps(&x.2, &y.2), "{} and {} overlap", x.0, y.0);
            }
        }
    }

    #[test]
    fn words_that_do_not_fit_are_counted_not_overlapped() {
        let mut c = WordCloudChart::new();
        for i in 0..120 {
            c = c.word(format!("word{i}"), 100.0 - i as f64 * 0.5);
        }
        let s = c.scene(240.0, 160.0, &InterMetrics).unwrap();
        let b = boxes(&s);
        assert!(b.len() < 120);
        assert!(s.label.contains("did not fit"), "{}", s.label);
        for (i, x) in b.iter().enumerate() {
            for y in &b[i + 1..] {
                assert!(!overlaps(&x.2, &y.2));
            }
        }
    }
}
