package schema

import (
	"reflect"
	"testing"
)

// fk is a tiny helper to build a foreign key referencing refTable.
func fk(name, refTable string) *ForeignKey {
	return &ForeignKey{Name: name, RefTable: refTable, Columns: []string{refTable + "_id"}, RefColumns: []string{"id"}}
}

func tableWithFKs(name string, fks ...*ForeignKey) *Table {
	return &Table{Name: name, ForeignKeys: fks}
}

func TestTopologicalSort_ParentsBeforeChildren(t *testing.T) {
	// orders -> users, order_items -> orders, order_items -> products
	s := New("shop").
		AddTable(&Table{Name: "users"}).
		AddTable(&Table{Name: "products"}).
		AddTable(tableWithFKs("orders", fk("fk_orders_user", "users"))).
		AddTable(tableWithFKs("order_items",
			fk("fk_items_order", "orders"),
			fk("fk_items_product", "products")))

	g := BuildGraph(s)
	order, cyclic := g.TopologicalSort()

	if len(cyclic) != 0 {
		t.Fatalf("expected no cycles, got %v", cyclic)
	}

	pos := indexOf(order)
	if pos["users"] > pos["orders"] {
		t.Errorf("users must come before orders: %v", order)
	}
	if pos["orders"] > pos["order_items"] {
		t.Errorf("orders must come before order_items: %v", order)
	}
	if pos["products"] > pos["order_items"] {
		t.Errorf("products must come before order_items: %v", order)
	}
}

func TestDropOrder_IsReverseOfCreation(t *testing.T) {
	s := New("shop").
		AddTable(&Table{Name: "users"}).
		AddTable(tableWithFKs("orders", fk("fk_orders_user", "users")))

	g := BuildGraph(s)
	create := g.CreationOrder()
	drop := g.DropOrder()

	if !reflect.DeepEqual(drop, []string{"orders", "users"}) {
		t.Errorf("expected drop order [orders users], got %v", drop)
	}
	if !reflect.DeepEqual(create, []string{"users", "orders"}) {
		t.Errorf("expected create order [users orders], got %v", create)
	}
}

func TestTopologicalSort_DetectsCycle(t *testing.T) {
	// a -> b and b -> a form a cycle.
	s := New("loop").
		AddTable(tableWithFKs("a", fk("fk_a_b", "b"))).
		AddTable(tableWithFKs("b", fk("fk_b_a", "a")))

	g := BuildGraph(s)
	order, cyclic := g.TopologicalSort()

	if len(order) != 0 {
		t.Errorf("expected nothing orderable, got %v", order)
	}
	if !reflect.DeepEqual(cyclic, []string{"a", "b"}) {
		t.Errorf("expected both tables flagged cyclic, got %v", cyclic)
	}
}

func TestBuildGraph_IgnoresSelfReference(t *testing.T) {
	// A tree table referencing its own parent_id must not be self-cyclic.
	s := New("tree").AddTable(tableWithFKs("nodes", fk("fk_parent", "nodes")))
	g := BuildGraph(s)
	order, cyclic := g.TopologicalSort()
	if len(cyclic) != 0 {
		t.Errorf("self-reference should not create a cycle, got %v", cyclic)
	}
	if !reflect.DeepEqual(order, []string{"nodes"}) {
		t.Errorf("expected [nodes], got %v", order)
	}
}

func indexOf(order []string) map[string]int {
	m := map[string]int{}
	for i, n := range order {
		m[n] = i
	}
	return m
}
