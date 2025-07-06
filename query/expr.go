// Package query provides an AST and helpers for building type-safe,
// composable Redisearch filter expressions.
//
//	import q "github.com/manojoshi/redisorm/query"
//
//	filter := q.And(
//	    q.Eq("status", "PENDING"),
//	    q.In("warehouse_id", 12, 15, 18),
//	    q.Not(q.Eq("is_deleted", 1)),
//	)
package query

import (
	"strings"
)

// -------------------------------------------------------------------
// Expr – the root interface. Every node knows how to write itself
// into a strings.Builder. We keep compile logic in compile.go so nodes
// stay dumb data containers.
// -------------------------------------------------------------------

type Expr interface {
	compile(*strings.Builder)
}

// ------------
// Leaf nodes
// ------------

// Eq("@field", value)  ➜  "@field:{value}"
func Eq(field string, v any) Expr { return &eq{field, v} }

// In("@field", v1, v2) ➜ "@field:{v1|v2}"
func In(field string, vs ...any) Expr { return &in{field, vs} }

// Range("@price", "[10 100]")  ➜ "@price:[10 100]"
func Range(field string, min, max any, inclusive bool) Expr {
	return &rng{field, min, max, inclusive}
}

// ------------
// Combinators
// ------------

func And(xs ...Expr) Expr { return &and{xs} } // implicit space
func Or(xs ...Expr) Expr  { return &or{xs} }  // |
func Not(x Expr) Expr     { return &not{x} }  // unary -

// -------------------------------------------------------------------
// internal node types
// -------------------------------------------------------------------

type (
	eq struct {
		f string
		v any
	}
	in struct {
		f  string
		vs []any
	}
	rng struct {
		f      string
		lo, hi any
		inc    bool
	}
	and struct{ xs []Expr }
	or  struct{ xs []Expr }
	not struct{ x Expr }
)

func field(f string) string {
	if strings.HasPrefix(f, "@") {
		return f
	}
	return "@" + f
}

func MatchAll() Expr { return matchAll{} }

type matchAll struct{}

func (matchAll) compile(sb *strings.Builder) { sb.WriteByte('*') }
