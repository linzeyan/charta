use super::frame::{self, LEGEND_SIZE, halo_label, text};
use super::{Chart, PAD, check_size, count, describe};
use crate::base::text::{TextMeasure, Weight};
use crate::base::theme::Color;
use crate::error::Error;
use crate::layout::layered::layered;
use crate::scene::{Anchor, Baseline, Corners, Hit, Mark, PathOp, Rect, Scene, Shape, TipRow};

const BOX_H: f64 = 28.0;
const DIAMOND_H: f64 = 40.0;
const BOX_PAD: f64 = 12.0;
const MIN_BOX_W: f64 = 64.0;
/// Horizontal room between neighbors in a row.
const H_GAP: f64 = 24.0;
/// Vertical room between rows: at least an arrowhead and a label; at most
/// enough that a short chart does not stretch into a ladder.
const MIN_V_GAP: f64 = 24.0;
const MAX_V_GAP: f64 = 40.0;
const ARROW_L: f64 = 7.0;
const ARROW_W: f64 = 8.0;
const EDGE_W: f64 = 1.5;
/// How far a loop back up runs to the right of the boxes it joins, plus
/// `LOOP_STEP` per extra row it spans: longer loops run outside shorter
/// ones, so nested loops never cross.
const LOOP: f64 = 24.0;
const LOOP_STEP: f64 = 16.0;
const CORNER: f64 = 8.0;
/// 4/3·(√2 − 1): cubic control distance for a quarter circle, per radius.
const KAPPA: f64 = 0.552_284_749_830_793_6;

/// The kind of a flow chart step, shown by its outline.
#[derive(Clone, Copy, Debug, Default, PartialEq, Eq)]
#[cfg_attr(feature = "serde", derive(serde::Serialize, serde::Deserialize))]
#[cfg_attr(feature = "serde", serde(rename_all = "lowercase"))]
pub enum StepKind {
    /// Start or end: a pill.
    Terminal,
    /// An action: a rounded box.
    #[default]
    Process,
    /// A question with several ways out: a diamond.
    Decision,
}

impl StepKind {
    fn name(self) -> &'static str {
        match self {
            StepKind::Terminal => "Terminal",
            StepKind::Process => "Process",
            StepKind::Decision => "Decision",
        }
    }
}

#[derive(Clone, Debug, PartialEq)]
#[cfg_attr(feature = "serde", derive(serde::Serialize, serde::Deserialize))]
pub struct Step {
    pub id: String,
    pub label: String,
    #[cfg_attr(feature = "serde", serde(default))]
    pub kind: StepKind,
}

/// An arrow from one step to the next, optionally labeled ("yes", "no").
#[derive(Clone, Debug, PartialEq)]
#[cfg_attr(feature = "serde", derive(serde::Serialize, serde::Deserialize))]
pub struct Arrow {
    pub from: String,
    pub to: String,
    #[cfg_attr(
        feature = "serde",
        serde(default, skip_serializing_if = "Option::is_none")
    )]
    pub label: Option<String>,
}

/// Steps joined by arrows, laid out top to bottom in rows so that arrows
/// point down; an arrow back up (a loop) curves around the right side.
///
/// # Example
///
/// ```
/// use charta::base::text::InterMetrics;
/// use charta::{Chart, FlowChart, StepKind};
///
/// let chart = FlowChart::new()
///     .title("Order handling")
///     .step("order", StepKind::Terminal, "Order placed")
///     .step("stock", StepKind::Decision, "In stock?")
///     .step("ship", StepKind::Process, "Ship")
///     .step("wait", StepKind::Process, "Wait for stock")
///     .arrow("order", "stock", "")
///     .arrow("stock", "ship", "Yes")
///     .arrow("stock", "wait", "No")
///     .arrow("wait", "stock", "");
/// let scene = chart.scene(640.0, 400.0, &InterMetrics).unwrap();
/// // The scene's label is what a screen reader announces.
/// assert_eq!(scene.label, "Order handling. Flow chart of 4 steps.");
/// ```
#[derive(Clone, Debug, Default, PartialEq)]
#[cfg_attr(feature = "serde", derive(serde::Serialize, serde::Deserialize))]
pub struct FlowChart {
    #[cfg_attr(
        feature = "serde",
        serde(default, skip_serializing_if = "Option::is_none")
    )]
    pub title: Option<String>,
    pub steps: Vec<Step>,
    pub arrows: Vec<Arrow>,
}

impl FlowChart {
    pub fn new() -> Self {
        FlowChart::default()
    }

    pub fn title(mut self, title: impl Into<String>) -> Self {
        self.title = Some(title.into());
        self
    }

    pub fn step(mut self, id: impl Into<String>, kind: StepKind, label: impl Into<String>) -> Self {
        self.steps.push(Step {
            id: id.into(),
            label: label.into(),
            kind,
        });
        self
    }

    /// An empty `label` means none.
    pub fn arrow(
        mut self,
        from: impl Into<String>,
        to: impl Into<String>,
        label: impl Into<String>,
    ) -> Self {
        let label: String = label.into();
        self.arrows.push(Arrow {
            from: from.into(),
            to: to.into(),
            label: (!label.is_empty()).then_some(label),
        });
        self
    }

    fn edges(&self) -> Result<Vec<(usize, usize)>, Error> {
        for (i, s) in self.steps.iter().enumerate() {
            if self.steps[..i].iter().any(|o| o.id == s.id) {
                return Err(Error::InvalidData(format!(
                    "step id {:?} is used twice",
                    s.id
                )));
            }
        }
        let find = |id: &str| {
            self.steps.iter().position(|s| s.id == id).ok_or_else(|| {
                Error::InvalidData(format!("an arrow names step {id:?}, which does not exist"))
            })
        };
        let mut edges: Vec<(usize, usize)> = Vec::with_capacity(self.arrows.len());
        for a in &self.arrows {
            let e = (find(&a.from)?, find(&a.to)?);
            if e.0 == e.1 {
                return Err(Error::InvalidData(format!(
                    "an arrow leads from {:?} to itself",
                    a.from
                )));
            }
            if edges.contains(&e) {
                return Err(Error::InvalidData(format!(
                    "the arrow {:?} → {:?} is given twice",
                    a.from, a.to
                )));
            }
            edges.push(e);
        }
        Ok(edges)
    }
}

impl Chart for FlowChart {
    fn scene(&self, width: f64, height: f64, measure: &dyn TextMeasure) -> Result<Scene, Error> {
        check_size(width, height)?;
        let edges = self.edges()?;
        let n = self.steps.len();
        let label = describe(
            self.title.as_deref(),
            &format!("Flow chart of {}.", count(n, "step", "steps")),
        );
        let mut marks = Vec::new();
        let top = frame::title(&mut marks, self.title.as_deref());
        if n == 0 {
            return Ok(Scene {
                width,
                height,
                label,
                marks,
            });
        }

        let size: Vec<(f64, f64)> = self
            .steps
            .iter()
            .map(|s| {
                let tw = measure.width(&s.label, LEGEND_SIZE, Weight::Regular);
                match s.kind {
                    StepKind::Process => ((tw + 2.0 * BOX_PAD).max(MIN_BOX_W), BOX_H),
                    StepKind::Terminal => {
                        ((tw + 2.0 * BOX_PAD + BOX_H / 2.0).max(MIN_BOX_W), BOX_H)
                    }
                    // Wide enough that the text's cap height, centered,
                    // clears the slanted sides with BOX_PAD / 2 to spare.
                    StepKind::Decision => (
                        ((tw + BOX_PAD) * DIAMOND_H / (DIAMOND_H - LEGEND_SIZE)).max(MIN_BOX_W),
                        DIAMOND_H,
                    ),
                }
            })
            .collect();
        let widths: Vec<f64> = size.iter().map(|s| s.0).collect();
        let l = layered(&widths, &edges, H_GAP);

        let mut row_h = vec![0.0; l.layers];
        for i in 0..n {
            row_h[l.layer[i]] = f64::max(row_h[l.layer[i]], size[i].1);
        }
        let avail_h = height - PAD - top;
        let rows_h: f64 = row_h.iter().sum();
        let v_gap = if l.layers > 1 {
            ((avail_h - rows_h) / (l.layers - 1) as f64).min(MAX_V_GAP)
        } else {
            0.0
        };
        if l.layers > 1 && v_gap < MIN_V_GAP {
            return Err(Error::InvalidData(format!(
                "{} of steps need {}px of height; the chart has {}px",
                count(l.layers, "row", "rows"),
                rows_h + MIN_V_GAP * (l.layers - 1) as f64,
                avail_h.max(0.0)
            )));
        }
        let span = |k: usize| {
            let (s, t) = edges[k];
            l.layer[s].abs_diff(l.layer[t])
        };
        let widest_loop = (0..edges.len())
            .filter(|&k| l.reversed[k])
            .map(|k| LOOP + LOOP_STEP * (span(k) - 1) as f64)
            .fold(0.0, f64::max);
        let need_w = l.width + widest_loop;
        let avail_w = width - 2.0 * PAD;
        if need_w > avail_w {
            return Err(Error::InvalidData(format!(
                "the widest row of steps needs {}px; the chart has {}px",
                need_w.ceil(),
                avail_w.max(0.0)
            )));
        }
        let ox = PAD + (avail_w - need_w) / 2.0;
        let mut row_y = Vec::with_capacity(l.layers);
        let mut y = top;
        for &h in &row_h {
            row_y.push(y);
            y += h + v_gap;
        }
        let cx = |i: usize| ox + l.x[i];
        let cy = |i: usize| row_y[l.layer[i]] + row_h[l.layer[i]] / 2.0;

        // Arrows first: boxes cover their ends.
        let mut labels: Vec<((f64, f64), &str)> = Vec::new();
        for (k, &(s, t)) in edges.iter().enumerate() {
            let mut ops;
            let mid;
            if l.reversed[k] {
                // s is below t: out of s's right side, up a vertical run
                // clear of both boxes, into t's right side pointing left.
                let start = (cx(s) + size[s].0 / 2.0, cy(s));
                let tip = (cx(t) + size[t].0 / 2.0, cy(t));
                let end = (tip.0 + ARROW_L, tip.1);
                let run = start.0.max(end.0) + LOOP + LOOP_STEP * (span(k) - 1) as f64;
                let (r, kr) = (CORNER, KAPPA * CORNER);
                ops = vec![
                    PathOp::Move(start.0, start.1),
                    PathOp::Line(run - r, start.1),
                    PathOp::Cubic(
                        run - r + kr,
                        start.1,
                        run,
                        start.1 - r + kr,
                        run,
                        start.1 - r,
                    ),
                    PathOp::Line(run, end.1 + r),
                    PathOp::Cubic(run, end.1 + r - kr, run - r + kr, end.1, run - r, end.1),
                    PathOp::Line(end.0, end.1),
                ];
                mid = (run, (start.1 + end.1) / 2.0);
                marks.push(Mark::new(Shape::Path { ops }).stroke(Color::TextMuted, EDGE_W));
                marks.push(
                    Mark::new(Shape::Path {
                        ops: vec![
                            PathOp::Move(tip.0, tip.1),
                            PathOp::Line(tip.0 + ARROW_L, tip.1 - ARROW_W / 2.0),
                            PathOp::Line(tip.0 + ARROW_L, tip.1 + ARROW_W / 2.0),
                            PathOp::Close,
                        ],
                    })
                    .fill(Color::TextMuted),
                );
            } else {
                let mut pts = vec![(cx(s), cy(s) + size[s].1 / 2.0)];
                for (b, &bx) in l.bends[k].iter().enumerate() {
                    let layer = l.layer[s] + 1 + b;
                    pts.push((ox + bx, row_y[layer]));
                    pts.push((ox + bx, row_y[layer] + row_h[layer]));
                }
                let tip = (cx(t), cy(t) - size[t].1 / 2.0);
                pts.push((tip.0, tip.1 - ARROW_L));
                ops = vec![PathOp::Move(pts[0].0, pts[0].1)];
                for w in pts.windows(2) {
                    let (p, q) = (w[0], w[1]);
                    if p.0 == q.0 {
                        ops.push(PathOp::Line(q.0, q.1));
                    } else {
                        let m = (p.1 + q.1) / 2.0;
                        ops.push(PathOp::Cubic(p.0, m, q.0, m, q.0, q.1));
                    }
                }
                // A label goes halfway along the first leg, where arrows
                // leaving the same step have already parted.
                mid = ((pts[0].0 + pts[1].0) / 2.0, (pts[0].1 + pts[1].1) / 2.0);
                marks.push(Mark::new(Shape::Path { ops }).stroke(Color::TextMuted, EDGE_W));
                marks.push(
                    Mark::new(Shape::Path {
                        ops: vec![
                            PathOp::Move(tip.0, tip.1),
                            PathOp::Line(tip.0 - ARROW_W / 2.0, tip.1 - ARROW_L),
                            PathOp::Line(tip.0 + ARROW_W / 2.0, tip.1 - ARROW_L),
                            PathOp::Close,
                        ],
                    })
                    .fill(Color::TextMuted),
                );
            }
            if let Some(lbl) = self.arrows[k].label.as_ref().filter(|l| !l.is_empty()) {
                labels.push((mid, lbl));
            }
        }

        for (i, step) in self.steps.iter().enumerate() {
            let (w, h) = size[i];
            let (x, y) = (cx(i), cy(i));
            let r = Rect {
                x: x - w / 2.0,
                y: y - h / 2.0,
                w,
                h,
            };
            let shape = match step.kind {
                StepKind::Decision => Shape::Path {
                    ops: vec![
                        PathOp::Move(x, r.y),
                        PathOp::Line(r.x + w, y),
                        PathOp::Line(x, r.y + h),
                        PathOp::Line(r.x, y),
                        PathOp::Close,
                    ],
                },
                StepKind::Terminal => Shape::Rect {
                    rect: r,
                    radius: Corners::all(h / 2.0),
                },
                StepKind::Process => Shape::Rect {
                    rect: r,
                    radius: Corners::all(4.0),
                },
            };
            // An opaque base under the wash, so a loop passing behind a box
            // does not show through it.
            marks.push(Mark::new(shape.clone()).fill(Color::Surface));
            let rows = edges
                .iter()
                .zip(&self.arrows)
                .filter(|((s, _), _)| *s == i)
                .map(|(&(_, t), a)| TipRow {
                    color: None,
                    name: a
                        .label
                        .clone()
                        .filter(|l| !l.is_empty())
                        .unwrap_or_else(|| "next".into()),
                    value: self.steps[t].label.clone(),
                })
                .collect();
            marks.push(
                Mark::new(shape)
                    .fill(Color::Wash(0))
                    .stroke(Color::Series(0), 1.0)
                    .hit(
                        Hit::new(format!("step-{i}"), step.label.as_str(), step.kind.name())
                            .rows(rows),
                    ),
            );
            marks.push(text(
                x,
                y,
                &step.label,
                LEGEND_SIZE,
                Weight::Regular,
                Anchor::Middle,
                Baseline::Middle,
                Color::TextPrimary,
            ));
        }
        for ((x, y), s) in labels {
            halo_label(
                &mut marks,
                measure,
                (x, y),
                s,
                Anchor::Middle,
                Color::TextSecondary,
            );
        }
        Ok(Scene {
            width,
            height,
            label,
            marks,
        })
    }
}

#[cfg(test)]
mod tests {
    use super::*;
    use crate::base::text::InterMetrics;
    use crate::scene::HitKind;

    fn order() -> FlowChart {
        FlowChart::new()
            .step("start", StepKind::Terminal, "Order received")
            .step("stock", StepKind::Decision, "In stock?")
            .step("ship", StepKind::Process, "Ship")
            .step("wait", StepKind::Process, "Restock")
            .step("end", StepKind::Terminal, "Done")
            .arrow("start", "stock", "")
            .arrow("stock", "ship", "Yes")
            .arrow("stock", "wait", "No")
            .arrow("wait", "stock", "")
            .arrow("ship", "end", "")
    }

    fn boxes(s: &Scene) -> Vec<(String, Rect)> {
        s.marks
            .iter()
            .filter_map(|m| {
                let h = m.hit.as_ref().filter(|h| h.kind == HitKind::Mark)?;
                let r = match &m.shape {
                    Shape::Rect { rect, .. } => *rect,
                    Shape::Path { ops } => {
                        let pts = crate::scene::flatten(ops).concat();
                        let (x0, x1) = pts
                            .iter()
                            .fold((f64::MAX, f64::MIN), |a, p| (a.0.min(p.0), a.1.max(p.0)));
                        let (y0, y1) = pts
                            .iter()
                            .fold((f64::MAX, f64::MIN), |a, p| (a.0.min(p.1), a.1.max(p.1)));
                        Rect {
                            x: x0,
                            y: y0,
                            w: x1 - x0,
                            h: y1 - y0,
                        }
                    }
                    _ => return None,
                };
                Some((h.label.clone(), r))
            })
            .collect()
    }

    #[test]
    fn steps_follow_the_arrows_downward_without_overlapping() {
        let s = order().scene(480.0, 360.0, &InterMetrics).unwrap();
        let b = boxes(&s);
        let at = |n: &str| b.iter().find(|x| x.0 == n).unwrap().1;
        for (from, to) in [
            ("Order received", "In stock?"),
            ("In stock?", "Ship"),
            ("Ship", "Done"),
        ] {
            assert!(at(from).y + at(from).h < at(to).y, "{from} above {to}");
        }
        for (i, (na, a)) in b.iter().enumerate() {
            for (nb, c) in &b[i + 1..] {
                let apart =
                    a.x + a.w <= c.x || c.x + c.w <= a.x || a.y + a.h <= c.y || c.y + c.h <= a.y;
                assert!(apart, "{na} overlaps {nb}");
            }
            assert!(a.x >= PAD - 1e-9 && a.x + a.w <= 480.0 - PAD + 1e-9, "{na}");
        }
    }

    #[test]
    fn a_decision_names_where_each_answer_leads() {
        let s = order().scene(480.0, 360.0, &InterMetrics).unwrap();
        let h = s
            .marks
            .iter()
            .find_map(|m| m.hit.as_ref().filter(|h| h.label == "In stock?"))
            .unwrap();
        assert_eq!(h.value, "Decision");
        let rows: Vec<(&str, &str)> = h
            .rows
            .iter()
            .map(|r| (r.name.as_str(), r.value.as_str()))
            .collect();
        assert_eq!(rows, [("Yes", "Ship"), ("No", "Restock")]);
    }

    #[test]
    fn a_loop_back_enters_from_the_right() {
        let s = order().scene(480.0, 360.0, &InterMetrics).unwrap();
        let stock = boxes(&s)
            .into_iter()
            .find(|b| b.0 == "In stock?")
            .unwrap()
            .1;
        // The left-pointing arrowhead's tip touches the diamond's right corner.
        let tip = s.marks.iter().find_map(|m| match &m.shape {
            Shape::Path { ops } if m.fill == Some(Color::TextMuted) => match (ops[0], ops[1]) {
                (PathOp::Move(x, y), PathOp::Line(x1, _)) if x1 > x => Some((x, y)),
                _ => None,
            },
            _ => None,
        });
        let (x, y) = tip.expect("a left-pointing arrowhead");
        assert!((x - (stock.x + stock.w)).abs() < 1e-9);
        assert!((y - (stock.y + stock.h / 2.0)).abs() < 1e-9);
    }

    #[test]
    fn a_longer_loop_runs_outside_a_shorter_one() {
        // "Ask for details" → "Investigate" (2 rows up) starts right of
        // where "Resolved?" → "Triage" (5 rows up) starts and ends: only the
        // per-row step keeps the long loop's run outside the short one's.
        let s = FlowChart::new()
            .step("open", StepKind::Terminal, "Ticket opened")
            .step("triage", StepKind::Process, "Triage")
            .step("known", StepKind::Decision, "Known issue?")
            .step("investigate", StepKind::Process, "Investigate")
            .step("repro", StepKind::Decision, "Reproduced?")
            .step("details", StepKind::Process, "Ask for details")
            .step("fix", StepKind::Process, "Send fix")
            .step("resolved", StepKind::Decision, "Resolved?")
            .step("close", StepKind::Terminal, "Close ticket")
            .arrow("open", "triage", "")
            .arrow("triage", "known", "")
            .arrow("known", "fix", "Yes")
            .arrow("known", "investigate", "No")
            .arrow("investigate", "repro", "")
            .arrow("repro", "fix", "Yes")
            .arrow("repro", "details", "No")
            .arrow("details", "investigate", "")
            .arrow("fix", "resolved", "")
            .arrow("resolved", "close", "Yes")
            .arrow("resolved", "triage", "No")
            .scene(640.0, 560.0, &InterMetrics)
            .unwrap();
        // Loops are the stroked paths with a vertical run as fourth op.
        let mut runs: Vec<(f64, f64)> = s
            .marks
            .iter()
            .filter_map(|m| match &m.shape {
                Shape::Path { ops } if m.stroke.is_some() && ops.len() == 6 => {
                    match (ops[2], ops[3]) {
                        (PathOp::Cubic(.., x, y0), PathOp::Line(x2, y1)) if x == x2 => {
                            Some((x, y0 - y1))
                        }
                        _ => None,
                    }
                }
                _ => None,
            })
            .collect();
        assert_eq!(runs.len(), 2);
        runs.sort_by(|a, b| a.1.total_cmp(&b.1));
        assert!(runs[1].0 > runs[0].0, "{runs:?}");
    }

    #[test]
    fn too_little_room_is_an_error_not_an_overlap() {
        assert!(order().scene(480.0, 120.0, &InterMetrics).is_err());
        assert!(order().scene(120.0, 360.0, &InterMetrics).is_err());
    }

    #[test]
    fn bad_arrows_are_rejected() {
        let m = &InterMetrics;
        let base = || {
            FlowChart::new()
                .step("a", StepKind::Process, "A")
                .step("b", StepKind::Process, "B")
        };
        assert!(base().arrow("a", "zz", "").scene(480.0, 360.0, m).is_err());
        assert!(base().arrow("a", "a", "").scene(480.0, 360.0, m).is_err());
        assert!(
            base()
                .arrow("a", "b", "")
                .arrow("a", "b", "x")
                .scene(480.0, 360.0, m)
                .is_err()
        );
        assert!(
            base()
                .step("a", StepKind::Process, "again")
                .scene(480.0, 360.0, m)
                .is_err()
        );
    }

    #[test]
    fn an_empty_label_is_no_label() {
        // JSON can say "label": "", which must not draw an empty halo.
        let none = FlowChart::new()
            .step("a", StepKind::Process, "A")
            .step("b", StepKind::Process, "B")
            .arrow("a", "b", "");
        let mut empty = none.clone();
        empty.arrows[0].label = Some(String::new());
        let scene = |c: &FlowChart| c.scene(640.0, 400.0, &InterMetrics).unwrap();
        assert_eq!(scene(&none), scene(&empty));
    }
}
