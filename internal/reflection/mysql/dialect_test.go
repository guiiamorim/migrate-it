package mysql

import (
	"strings"
	"testing"

	"github.com/guiiamorim/migrateit/internal/schema"
)

func TestDialect_CreateTableWithInlineFK(t *testing.T) {
	d := NewDialect()
	def := "0"
	tbl := &schema.Table{
		Name:    "orders",
		Engine:  "InnoDB",
		Charset: "utf8mb4",
		Columns: []*schema.Column{
			{Name: "id", Definition: "bigint", Nullable: false, Extra: "auto_increment"},
			{Name: "user_id", Definition: "bigint", Nullable: false},
			{Name: "status", Definition: "int", Nullable: false, Default: &def},
		},
		PrimaryKey: &schema.Constraint{Name: "PRIMARY", Type: PrimaryKey, Columns: []string{"id"}},
	}
	fk := &schema.ForeignKey{
		Name: "fk_orders_user", Columns: []string{"user_id"},
		RefTable: "users", RefColumns: []string{"id"}, OnDelete: "CASCADE",
	}

	got := d.CreateTable(tbl, []*schema.ForeignKey{fk})

	wants := []string{
		"CREATE TABLE `orders`",
		"`id` bigint NOT NULL AUTO_INCREMENT",
		"`status` int NOT NULL DEFAULT 0",
		"PRIMARY KEY (`id`)",
		"CONSTRAINT `fk_orders_user` FOREIGN KEY (`user_id`) REFERENCES `users` (`id`) ON DELETE CASCADE",
		"ENGINE=InnoDB",
		"DEFAULT CHARSET=utf8mb4",
	}
	for _, w := range wants {
		if !strings.Contains(got, w) {
			t.Errorf("CreateTable output missing %q\n--- got ---\n%s", w, got)
		}
	}
}

func TestDialect_AlterStatements(t *testing.T) {
	d := NewDialect()
	col := &schema.Column{Name: "email", Definition: "varchar(255)", Nullable: false}

	cases := map[string]string{
		d.AddColumn("users", col):    "ALTER TABLE `users` ADD COLUMN `email` varchar(255) NOT NULL",
		d.DropColumn("users", "old"): "ALTER TABLE `users` DROP COLUMN `old`",
		d.ModifyColumn("users", col): "ALTER TABLE `users` MODIFY COLUMN `email` varchar(255) NOT NULL",
		d.DropForeignKey("orders", &schema.ForeignKey{Name: "fk_orders_user"}): "ALTER TABLE `orders` DROP FOREIGN KEY `fk_orders_user`",
	}
	for got, want := range cases {
		if got != want {
			t.Errorf("got %q, want %q", got, want)
		}
	}
}

func TestDialect_IdentifierQuotingEscapesBackticks(t *testing.T) {
	if got := NewDialect().DropTable("we`ird"); got != "DROP TABLE `we``ird`" {
		t.Errorf("backtick not escaped: %s", got)
	}
}

func TestColumnType_MapsKnownAndUnknown(t *testing.T) {
	if got := columnType("varchar"); got != Varchar {
		t.Errorf("varchar -> %s, want %s", got, Varchar)
	}
	if got := columnType("BIGINT"); got != Bigint {
		t.Errorf("case-insensitive mapping failed: %s", got)
	}
	if got := columnType("geometry"); got != schema.ColumnType("GEOMETRY") {
		t.Errorf("unknown type should pass through upper-cased, got %s", got)
	}
}
