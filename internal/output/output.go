// Package output renders a schema diff in a requested format: an executable SQL
// migration script, the diff itself as JSON or YAML, or a git-diff-style HTML
// report.
package output

import (
	"encoding/json"
	"fmt"

	"github.com/guiiamorim/migrateit/internal/schema"
	"gopkg.in/yaml.v3"
)

// Supported output formats.
const (
	FormatSQL  = "sql"
	FormatJSON = "json"
	FormatYAML = "yaml"
	FormatHTML = "html"
)

// Default is the format used when none is specified.
const Default = FormatSQL

// Formats lists the formats a user may request (including not-yet-implemented
// ones, so the CLI can give an accurate error rather than "unknown format").
var Formats = []string{FormatSQL, FormatJSON, FormatYAML, FormatHTML}

// Validate reports whether format can be produced, without doing any work.
// It is meant to be called before connecting to any database so the CLI fails
// fast on a bad -format value.
func Validate(format string) error {
	switch format {
	case FormatSQL, FormatJSON, FormatYAML, FormatHTML:
		return nil
	default:
		return fmt.Errorf("unsupported format %q (supported: sql, json, yaml, html)", format)
	}
}

// Render produces the diff in the requested format.
//
// For SQL, dialect must be non-nil; it is the target database's dialect and
// determines the syntax of the generated migration script. JSON and YAML render
// the diff structurally and ignore dialect.
func Render(d *schema.Diff, format string, dialect schema.Dialect) ([]byte, error) {
	switch format {
	case FormatSQL:
		if dialect == nil {
			return nil, fmt.Errorf("sql output requires a target dialect")
		}
		return []byte(schema.BuildMigration(d, dialect).SQL()), nil
	case FormatJSON:
		b, err := json.MarshalIndent(d.Report(), "", "  ")
		if err != nil {
			return nil, err
		}
		return append(b, '\n'), nil
	case FormatYAML:
		return yaml.Marshal(d.Report())
	case FormatHTML:
		return renderHTML(d)
	default:
		// Validate gives the precise reason (unimplemented vs. unknown).
		if err := Validate(format); err != nil {
			return nil, err
		}
		return nil, fmt.Errorf("unsupported format %q", format)
	}
}
