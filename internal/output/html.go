package output

import (
	"bytes"
	"fmt"
	"html/template"
	"sort"
	"strings"
	"time"

	"github.com/guiiamorim/migrateit/internal/schema"
)

// --- view model -------------------------------------------------------------

type htmlReport struct {
	Source    string
	Target    string
	Generated string
	Summary   []summaryStat
	Tables    []htmlTable
	Empty     bool
}

type summaryStat struct {
	Label string
	Kind  string // css modifier: add | del | mod
}

type htmlTable struct {
	Name     string
	Status   string // created | dropped | modified
	Sections []htmlSection
}

type htmlSection struct {
	Title string
	Rows  []htmlRow
}

type htmlRow struct {
	Op  string // add | del | mod
	Old string
	New string
}

// --- rendering --------------------------------------------------------------

func renderHTML(d *schema.Diff) ([]byte, error) {
	report := buildHTMLReport(d)
	var buf bytes.Buffer
	if err := htmlTmpl.Execute(&buf, report); err != nil {
		return nil, fmt.Errorf("rendering html: %w", err)
	}
	return buf.Bytes(), nil
}

func buildHTMLReport(d *schema.Diff) htmlReport {
	byTable := map[string][]schema.Change{}
	for _, c := range d.Changes {
		byTable[c.Table] = append(byTable[c.Table], c)
	}

	names := make([]string, 0, len(byTable))
	for name := range byTable {
		names = append(names, name)
	}
	sort.Strings(names)

	var c counts
	report := htmlReport{
		Source:    d.Source.Name,
		Target:    d.Target.Name,
		Generated: time.Now().Format("2006-01-02 15:04 MST"),
	}

	for _, name := range names {
		changes := byTable[name]
		t := htmlTable{Name: name}
		switch {
		case hasKind(changes, schema.CreateTable):
			t.Status = "created"
			c.tCreate++
			t.Sections = tableSections(d.Target.Table(name), "add")
		case hasKind(changes, schema.DropTable):
			t.Status = "dropped"
			c.tDrop++
			t.Sections = tableSections(d.Source.Table(name), "del")
		default:
			t.Status = "modified"
			c.tMod++
			t.Sections = modifiedSections(changes, &c)
		}
		report.Tables = append(report.Tables, t)
	}

	report.Summary = c.chips()
	report.Empty = len(report.Tables) == 0
	return report
}

// tableSections renders every element of a whole table as a single operation
// (all additions for a created table, all deletions for a dropped one).
func tableSections(t *schema.Table, op string) []htmlSection {
	if t == nil {
		return nil
	}
	sig := func(s string) htmlRow {
		if op == "del" {
			return htmlRow{Op: "del", Old: s}
		}
		return htmlRow{Op: "add", New: s}
	}

	var cols, cons, fks []htmlRow
	for _, col := range t.Columns {
		cols = append(cols, sig(columnSignature(col)))
	}
	if t.PrimaryKey != nil {
		cons = append(cons, sig(constraintSignature(t.PrimaryKey)))
	}
	for _, k := range t.Constraints {
		cons = append(cons, sig(constraintSignature(k)))
	}
	for _, fk := range t.ForeignKeys {
		fks = append(fks, sig(foreignKeySignature(fk)))
	}
	return assembleSections(cols, cons, fks)
}

// modifiedSections renders the per-element changes of an altered table. Drop+add
// pairs on the same constraint or foreign-key name (which Compare emits for a
// changed element) are collapsed into a single modification row.
func modifiedSections(changes []schema.Change, c *counts) []htmlSection {
	var cols []htmlRow

	consAdd := map[string]*schema.Constraint{}
	consDel := map[string]*schema.Constraint{}
	var consOrder []string
	fkAdd := map[string]*schema.ForeignKey{}
	fkDel := map[string]*schema.ForeignKey{}
	var fkOrder []string
	seenCon := map[string]bool{}
	seenFK := map[string]bool{}

	for _, ch := range changes {
		switch ch.Kind {
		case schema.AddColumn:
			cols = append(cols, htmlRow{Op: "add", New: columnSignature(ch.Column)})
			c.cAdd++
		case schema.DropColumn:
			cols = append(cols, htmlRow{Op: "del", Old: columnSignature(ch.Column)})
			c.cDel++
		case schema.ModifyColumn:
			cols = append(cols, htmlRow{Op: "mod", Old: columnSignature(ch.OldColumn), New: columnSignature(ch.Column)})
			c.cMod++
		case schema.AddConstraint:
			consAdd[ch.Constraint.Name] = ch.Constraint
			if !seenCon[ch.Constraint.Name] {
				seenCon[ch.Constraint.Name] = true
				consOrder = append(consOrder, ch.Constraint.Name)
			}
		case schema.DropConstraint:
			consDel[ch.Constraint.Name] = ch.Constraint
			if !seenCon[ch.Constraint.Name] {
				seenCon[ch.Constraint.Name] = true
				consOrder = append(consOrder, ch.Constraint.Name)
			}
		case schema.AddForeignKey:
			fkAdd[ch.ForeignKey.Name] = ch.ForeignKey
			if !seenFK[ch.ForeignKey.Name] {
				seenFK[ch.ForeignKey.Name] = true
				fkOrder = append(fkOrder, ch.ForeignKey.Name)
			}
		case schema.DropForeignKey:
			fkDel[ch.ForeignKey.Name] = ch.ForeignKey
			if !seenFK[ch.ForeignKey.Name] {
				seenFK[ch.ForeignKey.Name] = true
				fkOrder = append(fkOrder, ch.ForeignKey.Name)
			}
		}
	}

	var cons []htmlRow
	for _, name := range consOrder {
		add, hasAdd := consAdd[name]
		del, hasDel := consDel[name]
		switch {
		case hasAdd && hasDel:
			cons = append(cons, htmlRow{Op: "mod", Old: constraintSignature(del), New: constraintSignature(add)})
			c.kMod++
		case hasAdd:
			cons = append(cons, htmlRow{Op: "add", New: constraintSignature(add)})
			c.kAdd++
		case hasDel:
			cons = append(cons, htmlRow{Op: "del", Old: constraintSignature(del)})
			c.kDel++
		}
	}

	var fks []htmlRow
	for _, name := range fkOrder {
		add, hasAdd := fkAdd[name]
		del, hasDel := fkDel[name]
		switch {
		case hasAdd && hasDel:
			fks = append(fks, htmlRow{Op: "mod", Old: foreignKeySignature(del), New: foreignKeySignature(add)})
			c.fMod++
		case hasAdd:
			fks = append(fks, htmlRow{Op: "add", New: foreignKeySignature(add)})
			c.fAdd++
		case hasDel:
			fks = append(fks, htmlRow{Op: "del", Old: foreignKeySignature(del)})
			c.fDel++
		}
	}

	return assembleSections(cols, cons, fks)
}

func assembleSections(cols, cons, fks []htmlRow) []htmlSection {
	var sections []htmlSection
	if len(cols) > 0 {
		sections = append(sections, htmlSection{Title: "Columns", Rows: cols})
	}
	if len(cons) > 0 {
		sections = append(sections, htmlSection{Title: "Indexes & Constraints", Rows: cons})
	}
	if len(fks) > 0 {
		sections = append(sections, htmlSection{Title: "Foreign Keys", Rows: fks})
	}
	return sections
}

func hasKind(changes []schema.Change, kind schema.ChangeKind) bool {
	for _, c := range changes {
		if c.Kind == kind {
			return true
		}
	}
	return false
}

// --- signatures (human-readable, dialect-neutral) ---------------------------

func columnSignature(c *schema.Column) string {
	var b strings.Builder
	b.WriteString(c.Name)
	if c.Definition != "" {
		b.WriteString(" ")
		b.WriteString(c.Definition)
	}
	if c.Nullable {
		b.WriteString(" NULL")
	} else {
		b.WriteString(" NOT NULL")
	}
	if c.Default != nil {
		b.WriteString(" DEFAULT ")
		b.WriteString(*c.Default)
	}
	if c.Extra != "" {
		b.WriteString(" ")
		b.WriteString(c.Extra)
	}
	return b.String()
}

func constraintSignature(c *schema.Constraint) string {
	label := strings.TrimSpace(string(c.Type))
	if c.Expression != "" {
		return fmt.Sprintf("%s %s CHECK (%s)", label, c.Name, c.Expression)
	}
	cols := strings.Join(c.Columns, ", ")
	// Primary keys rarely carry a meaningful user-facing name; omit it.
	if strings.EqualFold(label, "PRIMARY KEY") {
		return fmt.Sprintf("%s (%s)", label, cols)
	}
	return fmt.Sprintf("%s %s (%s)", label, c.Name, cols)
}

func foreignKeySignature(fk *schema.ForeignKey) string {
	var b strings.Builder
	fmt.Fprintf(&b, "%s FOREIGN KEY (%s) REFERENCES %s (%s)",
		fk.Name, strings.Join(fk.Columns, ", "), fk.RefTable, strings.Join(fk.RefColumns, ", "))
	if fk.OnDelete != "" {
		fmt.Fprintf(&b, " ON DELETE %s", fk.OnDelete)
	}
	if fk.OnUpdate != "" {
		fmt.Fprintf(&b, " ON UPDATE %s", fk.OnUpdate)
	}
	return b.String()
}

// --- summary counts ---------------------------------------------------------

type counts struct {
	tCreate, tDrop, tMod int
	cAdd, cDel, cMod     int
	kAdd, kDel, kMod     int
	fAdd, fDel, fMod     int
}

func (c counts) chips() []summaryStat {
	var out []summaryStat
	add := func(n int, noun, verb, kind string) {
		if n > 0 {
			out = append(out, summaryStat{Label: fmt.Sprintf("%d %s %s", n, plural(noun, n), verb), Kind: kind})
		}
	}
	add(c.tCreate, "table", "added", "add")
	add(c.tMod, "table", "modified", "mod")
	add(c.tDrop, "table", "removed", "del")
	add(c.cAdd, "column", "added", "add")
	add(c.cMod, "column", "modified", "mod")
	add(c.cDel, "column", "removed", "del")
	add(c.kAdd, "constraint", "added", "add")
	add(c.kMod, "constraint", "modified", "mod")
	add(c.kDel, "constraint", "removed", "del")
	add(c.fAdd, "foreign key", "added", "add")
	add(c.fMod, "foreign key", "modified", "mod")
	add(c.fDel, "foreign key", "removed", "del")
	return out
}

func plural(noun string, n int) string {
	if n == 1 {
		return noun
	}
	if noun == "foreign key" {
		return "foreign keys"
	}
	return noun + "s"
}

// --- template ---------------------------------------------------------------

var htmlTmpl = template.Must(template.New("report").Parse(htmlTemplate))

const htmlTemplate = `<!doctype html>
<html lang="en">
<head>
<meta charset="utf-8">
<meta name="viewport" content="width=device-width, initial-scale=1">
<title>Schema Diff — {{.Source}} → {{.Target}}</title>
<style>
:root{
  --bg:#f6f8fa;--card:#fff;--border:#d0d7de;--text:#1f2328;--muted:#656d76;
  --add-bg:#e6ffec;--add-border:#2da44e;--add-text:#116329;
  --del-bg:#ffebe9;--del-border:#cf222e;--del-text:#82071e;
  --mod-bg:#fff8c5;--mod-border:#d4a72c;--mod-text:#7d4e00;
}
@media (prefers-color-scheme:dark){
  :root{
    --bg:#0d1117;--card:#161b22;--border:#30363d;--text:#e6edf3;--muted:#8b949e;
    --add-bg:#12261e;--add-border:#238636;--add-text:#3fb950;
    --del-bg:#25171c;--del-border:#da3633;--del-text:#f85149;
    --mod-bg:#272115;--mod-border:#9e6a03;--mod-text:#d29922;
  }
}
*{box-sizing:border-box}
body{margin:0;background:var(--bg);color:var(--text);
  font-family:-apple-system,BlinkMacSystemFont,"Segoe UI",Helvetica,Arial,sans-serif;font-size:14px;line-height:1.5}
.container{max-width:980px;margin:0 auto;padding:40px 20px 72px}
.title{display:flex;align-items:center;gap:12px;font-size:24px;font-weight:600}
.logo{display:inline-flex;align-items:center;justify-content:center;width:34px;height:34px;border-radius:8px;
  background:linear-gradient(135deg,var(--add-border),#0969da);color:#fff;font-weight:700;font-size:18px}
.subtitle{color:var(--muted);margin-top:8px}
.flow{font-family:ui-monospace,SFMono-Regular,Menlo,Consolas,monospace;background:var(--card);
  border:1px solid var(--border);border-radius:6px;padding:2px 8px;font-size:13px}
.arrow{margin:0 6px;color:var(--muted)}
.summary{display:flex;flex-wrap:wrap;gap:8px;margin-top:20px}
.chip{font-size:12px;font-weight:600;padding:4px 12px;border-radius:999px;border:1px solid var(--border);background:var(--card)}
.chip.add{color:var(--add-text);border-color:var(--add-border);background:var(--add-bg)}
.chip.del{color:var(--del-text);border-color:var(--del-border);background:var(--del-bg)}
.chip.mod{color:var(--mod-text);border-color:var(--mod-border);background:var(--mod-bg)}
.card{background:var(--card);border:1px solid var(--border);border-radius:10px;margin-top:18px;overflow:hidden;
  box-shadow:0 1px 3px rgba(0,0,0,.05)}
.card>.head{display:flex;align-items:center;justify-content:space-between;gap:12px;padding:12px 16px;
  border-bottom:1px solid var(--border)}
.tname{font-family:ui-monospace,SFMono-Regular,Menlo,Consolas,monospace;font-weight:600;font-size:15px}
.badge{font-size:11px;text-transform:uppercase;letter-spacing:.05em;font-weight:700;padding:3px 10px;border-radius:999px}
.badge.created{background:var(--add-bg);color:var(--add-text)}
.badge.dropped{background:var(--del-bg);color:var(--del-text)}
.badge.modified{background:var(--mod-bg);color:var(--mod-text)}
.section+.section{border-top:1px solid var(--border)}
.section h3{font-size:11px;text-transform:uppercase;letter-spacing:.06em;color:var(--muted);
  margin:0;padding:12px 16px 6px}
.diff{font-family:ui-monospace,SFMono-Regular,Menlo,Consolas,monospace;font-size:13px;padding-bottom:8px}
.line{display:flex;gap:10px;padding:2px 16px;white-space:pre-wrap;word-break:break-word;border-left:3px solid transparent}
.line .gutter{user-select:none;flex:0 0 1ch;text-align:center;color:var(--muted)}
.line.add{background:var(--add-bg);border-left-color:var(--add-border)}
.line.add .gutter{color:var(--add-text)}
.line.del{background:var(--del-bg);border-left-color:var(--del-border)}
.line.del .gutter{color:var(--del-text)}
.empty{text-align:center;padding:56px 24px;color:var(--muted);font-size:16px}
footer{text-align:center;color:var(--muted);font-size:12px;margin-top:28px}
</style>
</head>
<body>
<div class="container">
  <header>
    <div class="title"><span class="logo">±</span><span>Schema Diff</span></div>
    <div class="subtitle">
      <span class="flow">{{.Source}}</span><span class="arrow">→</span><span class="flow">{{.Target}}</span>
      &nbsp;·&nbsp; generated {{.Generated}}
    </div>
    {{if .Summary}}<div class="summary">{{range .Summary}}<span class="chip {{.Kind}}">{{.Label}}</span>{{end}}</div>{{end}}
  </header>

  {{if .Empty}}
    <div class="card"><div class="empty">✓ No differences — the schemas are identical.</div></div>
  {{else}}
    {{range .Tables}}
    <section class="card">
      <div class="head"><span class="tname">{{.Name}}</span><span class="badge {{.Status}}">{{.Status}}</span></div>
      {{range .Sections}}
      <div class="section">
        <h3>{{.Title}}</h3>
        <div class="diff">
          {{range .Rows}}
            {{if eq .Op "mod"}}
              <div class="line del"><span class="gutter">-</span><span>{{.Old}}</span></div>
              <div class="line add"><span class="gutter">+</span><span>{{.New}}</span></div>
            {{else if eq .Op "del"}}
              <div class="line del"><span class="gutter">-</span><span>{{.Old}}</span></div>
            {{else}}
              <div class="line add"><span class="gutter">+</span><span>{{.New}}</span></div>
            {{end}}
          {{end}}
        </div>
      </div>
      {{end}}
    </section>
    {{end}}
  {{end}}

  <footer>Generated by migrate-it</footer>
</div>
</body>
</html>
`
