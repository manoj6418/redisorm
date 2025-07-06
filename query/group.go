package query

import "strings"

type GroupKey struct {
	raw   string
	alias string
}

func By(field string) GroupKey {
	if !strings.HasPrefix(field, "@") {
		field = "@" + field
	}
	return GroupKey{raw: field}
}

func ByExpr(expr string) GroupKey { return GroupKey{raw: expr} }

func (g GroupKey) As(alias string) GroupKey { g.alias = alias; return g }
