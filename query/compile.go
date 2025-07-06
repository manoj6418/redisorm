package query

import (
	"fmt"
	"strconv"
	"strings"
)

// Compile turns an Expr tree into a RediSearch query string.
// It is intentionally exported so callers can pre-view the query
// (handy for logging, metrics, or offline explain).
func Compile(e Expr) string {
	var sb strings.Builder
	e.compile(&sb)
	return sb.String()
}

// -------------------------------------------------------------------
// node writers – kept in a central file so cross-node helpers don’t
// cause import cycles. Only expr.go’s structs know about these funcs.
// -------------------------------------------------------------------

func (n *eq) compile(sb *strings.Builder) {
	fmt.Fprintf(sb, "%s:{%v}", field(n.f), n.v)
}

func (n *in) compile(sb *strings.Builder) {
	sb.WriteString(field(n.f) + ":{")
	for i, v := range n.vs {
		if i > 0 {
			sb.WriteByte('|')
		}
		fmt.Fprint(sb, v)
	}
	sb.WriteByte('}')
}

func (n *rng) compile(sb *strings.Builder) {
	left, right := "(", ")"
	if n.inc {
		left, right = "[", "]"
	}
	fmt.Fprintf(sb, "%s:%s%v %v%s", field(n.f), left, n.lo, n.hi, right)
}

func (n *and) compile(sb *strings.Builder) { group(sb, n.xs, " ") }
func (n *or) compile(sb *strings.Builder)  { group(sb, n.xs, "|") }

func (n *not) compile(sb *strings.Builder) {
	sb.WriteByte('-')
	sb.WriteByte('(')
	n.x.compile(sb)
	sb.WriteByte(')')
}

// group helper for (a b) / (a|b)
func group(sb *strings.Builder, xs []Expr, sep string) {
	sb.WriteByte('(')
	for i, x := range xs {
		if i > 0 {
			sb.WriteString(sep)
		}
		x.compile(sb)
	}
	sb.WriteByte(')')
}

// -------------------------------------------------------------------
// Small utility: convert any int-like to string *without* reflection.
// -------------------------------------------------------------------

func toStr(v any) string {
	switch t := v.(type) {
	case string:
		return t
	case int:
		return strconv.Itoa(t)
	case int64:
		return strconv.FormatInt(t, 10)
	default:
		return fmt.Sprint(t)
	}
}
