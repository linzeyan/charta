// Package sysfont finds the system's fonts in pure Go, the way the Rust
// crate's raster backend finds them with fontdb: Inter when installed, else
// the platform's sans-serif, regular and semibold, and per character the
// first face that has the glyph, so CJK labels render instead of leaving
// gaps. charta embeds no font (see spec), and Go has no fontdb.
//
// Only the name tables are read to index the fonts (sfnt.ParseReaderAt);
// a face's file stays open once it is picked, so glyphs load on demand
// without reading multi-megabyte CJK collections into memory.
package sysfont

import (
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"sync"

	"golang.org/x/image/font/sfnt"
)

// face is one installed face: where it is and what it is called.
type face struct {
	path   string
	index  int // within a collection; 0 for a single font
	family string
	weight int // CSS weight from the style name; 400 when unknown
}

type index struct {
	faces []face
	// open holds the faces parsed so far, by path and index; nil when the
	// face could not be parsed.
	mu   sync.Mutex
	open map[[2]any]*sfnt.Font
}

// sans are the families tried for text, in order: Inter first, as for the
// Inter width table the HTML output is measured with.
var sans = []string{
	"Inter",
	"Helvetica Neue", "Helvetica", // macOS
	"Segoe UI", "Arial", // Windows
	"Noto Sans", "DejaVu Sans", "Liberation Sans", "Cantarell", // Linux
}

// cjk are the families tried for characters the text face lacks, before
// every other face: the Rust crate's list.
var cjk = []string{
	"PingFang TC", "PingFang SC", "Hiragino Sans",
	"Noto Sans CJK TC", "Noto Sans CJK SC",
	"Microsoft JhengHei", "Microsoft YaHei",
	"Apple SD Gothic Neo",
}

var all = sync.OnceValue(func() *index {
	ix := &index{open: map[[2]any]*sfnt.Font{}}
	for _, dir := range dirs() {
		_ = filepath.WalkDir(dir, func(p string, d os.DirEntry, err error) error {
			if err != nil || d.IsDir() {
				return nil
			}
			switch strings.ToLower(filepath.Ext(p)) {
			case ".ttf", ".otf", ".ttc", ".otc":
				ix.faces = append(ix.faces, scan(p)...)
			}
			return nil
		})
	}
	return ix
})

func dirs() []string {
	home, _ := os.UserHomeDir()
	switch runtime.GOOS {
	case "darwin":
		return []string{"/System/Library/Fonts", "/Library/Fonts", filepath.Join(home, "Library/Fonts")}
	case "windows":
		return []string{filepath.Join(os.Getenv("WINDIR"), "Fonts"), filepath.Join(os.Getenv("LOCALAPPDATA"), "Microsoft/Windows/Fonts")}
	default:
		return []string{"/usr/share/fonts", "/usr/local/share/fonts", filepath.Join(home, ".local/share/fonts"), filepath.Join(home, ".fonts")}
	}
}

// scan lists the faces in one font file, skipping italics: chart text is
// upright.
func scan(path string) []face {
	f, err := os.Open(path)
	if err != nil {
		return nil
	}
	defer f.Close()
	c, err := sfnt.ParseCollectionReaderAt(f)
	if err != nil {
		return nil
	}
	var out []face
	var buf sfnt.Buffer
	for i := range c.NumFonts() {
		fnt, err := c.Font(i)
		if err != nil {
			continue
		}
		family := name(fnt, &buf, sfnt.NameIDTypographicFamily, sfnt.NameIDFamily)
		style := strings.ToLower(name(fnt, &buf, sfnt.NameIDTypographicSubfamily, sfnt.NameIDSubfamily))
		if family == "" || strings.Contains(style, "italic") || strings.Contains(style, "oblique") {
			continue
		}
		out = append(out, face{path: path, index: i, family: family, weight: weight(style)})
	}
	return out
}

// name is the first of the name IDs the font has: typographic names group
// every weight under one family, where legacy names split them.
func name(f *sfnt.Font, buf *sfnt.Buffer, ids ...sfnt.NameID) string {
	for _, id := range ids {
		if s, err := f.Name(buf, id); err == nil && s != "" {
			return s
		}
	}
	return ""
}

// weight reads a CSS weight off a style name such as "SemiBold" or
// "Demi Bold"; anything unrecognized counts as regular.
func weight(style string) int {
	s := strings.NewReplacer(" ", "", "-", "", "_", "").Replace(style)
	// Japanese faces number their weights: Hiragino W0 (hairline) to W9.
	if len(s) == 2 && s[0] == 'w' && s[1] >= '0' && s[1] <= '9' {
		return max(int(s[1]-'0'), 1) * 100
	}
	for _, w := range []struct {
		names []string
		w     int
	}{
		{[]string{"thin", "hairline"}, 100},
		{[]string{"extralight", "ultralight"}, 200},
		{[]string{"semibold", "demibold", "demi"}, 600},
		{[]string{"extrabold", "ultrabold"}, 800},
		{[]string{"black", "heavy"}, 900},
		{[]string{"bold"}, 700},
		{[]string{"medium"}, 500},
		{[]string{"light"}, 300},
	} {
		for _, n := range w.names {
			if strings.Contains(s, n) {
				return w.w
			}
		}
	}
	return 400
}

func (ix *index) load(fc face) *sfnt.Font {
	ix.mu.Lock()
	defer ix.mu.Unlock()
	key := [2]any{fc.path, fc.index}
	if f, ok := ix.open[key]; ok {
		return f
	}
	var fnt *sfnt.Font
	// Kept open for the life of the program: glyphs load on demand.
	if file, err := os.Open(fc.path); err == nil {
		if c, err := sfnt.ParseCollectionReaderAt(file); err == nil {
			fnt, _ = c.Font(fc.index)
		}
		if fnt == nil {
			file.Close()
		}
	}
	ix.open[key] = fnt
	return fnt
}

// pick is the face of family closest to want, by the CSS font matching
// order: for a bold-ish want, heavier weights ascending, then lighter
// descending; for regular, 400, then 500, then lighter, then heavier.
func (ix *index) pick(family string, want int) (face, bool) {
	var faces []face
	for _, f := range ix.faces {
		if strings.EqualFold(f.family, family) {
			faces = append(faces, f)
		}
	}
	if len(faces) == 0 {
		return face{}, false
	}
	rank := func(w int) int {
		switch {
		case w == want:
			return 0
		case want > 500 && w > want:
			return w - want
		case want > 500:
			return 1000 + want - w
		case want <= 500 && w > want && w <= 500:
			return w - want
		case w < want:
			return 1000 + want - w
		default:
			return 2000 + w - want
		}
	}
	return slices.MinFunc(faces, func(a, b face) int { return rank(a.weight) - rank(b.weight) }), true
}

// Sans is the system's text face in weight 400 or 600 (semibold: the
// closest heavier face when there is no 600): Inter when installed, else the
// platform's sans-serif. Nil when none is found.
func Sans(semibold bool) *sfnt.Font {
	ix := all()
	want := 400
	if semibold {
		want = 600
	}
	for _, fam := range sans {
		if fc, ok := ix.pick(fam, want); ok {
			if f := ix.load(fc); f != nil {
				return f
			}
		}
	}
	return nil
}

// Fallback is the first face with a glyph for r: the CJK families, then
// every other installed face. Nil when nothing installed has it. Callers
// cache per character; a miss scans every face once.
func Fallback(r rune) *sfnt.Font {
	ix := all()
	var buf sfnt.Buffer
	has := func(f *sfnt.Font) bool {
		x, err := f.GlyphIndex(&buf, r)
		return err == nil && x != 0
	}
	for _, fam := range cjk {
		if fc, ok := ix.pick(fam, 400); ok {
			if f := ix.load(fc); f != nil && has(f) {
				return f
			}
		}
	}
	for _, fc := range ix.faces {
		if probe(fc, r) {
			return ix.load(fc)
		}
	}
	return nil
}

// probe reports whether the face has a glyph for r, without keeping its
// file open: a scan touches every installed face.
func probe(fc face, r rune) bool {
	file, err := os.Open(fc.path)
	if err != nil {
		return false
	}
	defer file.Close()
	c, err := sfnt.ParseCollectionReaderAt(file)
	if err != nil {
		return false
	}
	f, err := c.Font(fc.index)
	if err != nil {
		return false
	}
	var buf sfnt.Buffer
	x, err := f.GlyphIndex(&buf, r)
	return err == nil && x != 0
}
