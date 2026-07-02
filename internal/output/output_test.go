package output

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"

	"github.com/guiiamorim/migrateit/internal/schema"
	"gopkg.in/yaml.v3"
)

// sampleDiff builds a small diff exercising several change kinds: a new table,
// a new column on an existing table, and a foreign key added to an existing
// table (a new table's own FKs are carried inside its CreateTable change, so an
// existing table is needed to produce a standalone AddForeignKey).
func sampleDiff() *schema.Diff {
	src := schema.New("old").
		AddTable(&schema.Table{Name: "users",
			Columns: []*schema.Column{{Name: "id", Definition: "bigint"}}}).
		AddTable(&schema.Table{Name: "orders",
			Columns: []*schema.Column{{Name: "id", Definition: "bigint"}, {Name: "user_id", Definition: "bigint"}}})
	tgt := schema.New("new").
		AddTable(&schema.Table{Name: "users",
			Columns: []*schema.Column{{Name: "id", Definition: "bigint"}, {Name: "email", Definition: "varchar(255)"}}}).
		AddTable(&schema.Table{Name: "orders",
			Columns:     []*schema.Column{{Name: "id", Definition: "bigint"}, {Name: "user_id", Definition: "bigint"}},
			ForeignKeys: []*schema.ForeignKey{{Name: "fk_orders_user", Columns: []string{"user_id"}, RefTable: "users", RefColumns: []string{"id"}}}}).
		AddTable(&schema.Table{Name: "products",
			Columns: []*schema.Column{{Name: "id", Definition: "bigint"}}})
	return schema.Compare(src, tgt)
}

// stubDialect is a minimal dialect so SQL rendering can be exercised without a
// real driver.
type stubDialect struct{}

func (stubDialect) CreateTable(t *schema.Table, _ []*schema.ForeignKey) string {
	return "CREATE " + t.Name
}
func (stubDialect) DropTable(t string) string                   { return "DROP " + t }
func (stubDialect) AddColumn(t string, c *schema.Column) string { return "ADDCOL " + t + "." + c.Name }
func (stubDialect) DropColumn(t, c string) string               { return "DROPCOL " + t + "." + c }
func (stubDialect) ModifyColumn(t string, c *schema.Column) string {
	return "MODCOL " + t + "." + c.Name
}
func (stubDialect) AddConstraint(t string, c *schema.Constraint) string  { return "ADDCON " + t }
func (stubDialect) DropConstraint(t string, c *schema.Constraint) string { return "DROPCON " + t }
func (stubDialect) AddForeignKey(t string, f *schema.ForeignKey) string {
	return "ADDFK " + t + "." + f.Name
}
func (stubDialect) DropForeignKey(t string, f *schema.ForeignKey) string {
	return "DROPFK " + t + "." + f.Name
}

func TestRenderSQL(t *testing.T) {
	got, err := Render(sampleDiff(), FormatSQL, stubDialect{})
	if err != nil {
		t.Fatal(err)
	}
	s := string(got)
	if !strings.Contains(s, "CREATE products") || !strings.Contains(s, "ADDCOL users.email") || !strings.Contains(s, "ADDFK orders.fk_orders_user") {
		t.Errorf("unexpected SQL output:\n%s", s)
	}
}

func TestRenderSQLRequiresDialect(t *testing.T) {
	if _, err := Render(sampleDiff(), FormatSQL, nil); err == nil {
		t.Error("expected error when dialect is nil for sql output")
	}
}

func TestRenderJSON(t *testing.T) {
	got, err := Render(sampleDiff(), FormatJSON, nil)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.HasSuffix(got, []byte("\n")) {
		t.Error("json output should end with a newline")
	}

	var report schema.DiffReport
	if err := json.Unmarshal(got, &report); err != nil {
		t.Fatalf("json is not valid: %v", err)
	}
	if report.Source != "old" || report.Target != "new" {
		t.Errorf("source/target not encoded: %+v", report)
	}
	if len(report.Changes) == 0 {
		t.Fatal("expected changes in report")
	}
	// The FK add must carry its reference details.
	var foundFK bool
	for _, c := range report.Changes {
		if c.Kind == schema.AddForeignKey.String() {
			foundFK = true
			if c.ForeignKey == nil || c.ForeignKey.RefTable != "users" {
				t.Errorf("foreign key change missing details: %+v", c)
			}
		}
	}
	if !foundFK {
		t.Error("expected an ADD FOREIGN KEY change in the report")
	}
}

func TestRenderYAML(t *testing.T) {
	got, err := Render(sampleDiff(), FormatYAML, nil)
	if err != nil {
		t.Fatal(err)
	}
	var report schema.DiffReport
	if err := yaml.Unmarshal(got, &report); err != nil {
		t.Fatalf("yaml is not valid: %v", err)
	}
	if report.Target != "new" || len(report.Changes) == 0 {
		t.Errorf("unexpected yaml report: %+v", report)
	}
}

func TestRenderHTML(t *testing.T) {
	if err := Validate(FormatHTML); err != nil {
		t.Fatalf("html should be a valid format: %v", err)
	}
	got, err := Render(sampleDiff(), FormatHTML, nil) // dialect not needed for html
	if err != nil {
		t.Fatal(err)
	}
	s := string(got)

	wants := []string{
		"<!doctype html>",
		"Schema Diff",
		`<span class="badge created">created</span>`, // products is a new table
		`<span class="badge modified">modified</span>`,
		"email varchar(255) NOT NULL",                                // added column signature
		"fk_orders_user FOREIGN KEY (user_id) REFERENCES users (id)", // added fk signature
		"Foreign Keys",
	}
	for _, w := range wants {
		if !strings.Contains(s, w) {
			t.Errorf("html output missing %q", w)
		}
	}
	// The diff-line classes must be present so styling actually applies.
	if !strings.Contains(s, `class="line add"`) {
		t.Error("expected at least one added diff line")
	}
}

func TestRenderHTMLModifiedColumnShowsBeforeAndAfter(t *testing.T) {
	src := schema.New("old").AddTable(&schema.Table{Name: "users",
		Columns: []*schema.Column{{Name: "name", Definition: "varchar(100)"}}})
	tgt := schema.New("new").AddTable(&schema.Table{Name: "users",
		Columns: []*schema.Column{{Name: "name", Definition: "varchar(255)"}}})

	got, err := Render(schema.Compare(src, tgt), FormatHTML, nil)
	if err != nil {
		t.Fatal(err)
	}
	s := string(got)
	// A modified column renders both the old (del) and new (add) signature.
	if !strings.Contains(s, "name varchar(100)") || !strings.Contains(s, "name varchar(255)") {
		t.Errorf("modified column should show before and after:\n%s", s)
	}
}

func TestRenderHTMLEmptyDiff(t *testing.T) {
	same := func() *schema.Schema {
		return schema.New("db").AddTable(&schema.Table{Name: "t",
			Columns: []*schema.Column{{Name: "id", Definition: "bigint"}}})
	}
	got, err := Render(schema.Compare(same(), same()), FormatHTML, nil)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(got), "No differences") {
		t.Error("empty diff should render a friendly identical-schemas message")
	}
}

func TestValidateUnknownFormat(t *testing.T) {
	if err := Validate("xml"); err == nil {
		t.Error("expected error for unknown format")
	}
}
