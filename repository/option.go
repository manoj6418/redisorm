package repository

import q "github.com/manojoshi/redisorm/query"

// Opt is applied to whichever builder is in play.  If the helper doesn’t make
// sense for that builder the method is left nil and becomes a no-op.
type Opt interface {
	applySearch(*q.SearchBuilder)
	applyAgg(*q.AggregateBuilder)
}

// optFunc is a concrete Opt implementation that holds functions for
type optFunc struct {
	search func(*q.SearchBuilder)
	agg    func(*q.AggregateBuilder)
}

// applySearch applies the Opt to the SearchBuilder if it is not nil.
func (o optFunc) applySearch(b *q.SearchBuilder) {
	if o.search != nil {
		o.search(b)
	}
}

// applyAgg applies the Opt to the AggregateBuilder if it is not nil.
func (o optFunc) applyAgg(b *q.AggregateBuilder) {
	if o.agg != nil {
		o.agg(b)
	}
}

// ---------- COMMON helpers ----------

// Select applies a list of fields to be returned by FT.SEARCH or FT.AGGREGATE.
func Select(fields ...string) Opt {
	return optFunc{
		search: func(b *q.SearchBuilder) { b.Select(fields...) },
	}
}

// Limit applies a limit to the number of results returned by FT.SEARCH or FT.AGGREGATE.
func Limit(offset, limit int) Opt {
	return optFunc{
		search: func(b *q.SearchBuilder) { b.Limit(offset, limit) },
		agg:    func(b *q.AggregateBuilder) { b.Limit(offset, limit) },
	}
}

// SortAsc SORT
func SortAsc(field string) Opt  { return sortOpt(field, q.Asc) }
func SortDesc(field string) Opt { return sortOpt(field, q.Desc) }

func sortOpt(f string, dir q.Dir) Opt {
	return optFunc{
		search: func(b *q.SearchBuilder) { b.SortBy(f, dir) },
	}
}

// AGGREGATE-only helpers

func Group(keys ...q.GroupKey) Opt {
	return optFunc{
		agg: func(b *q.AggregateBuilder) { b.GroupBy(keys...) },
	}
}

func Count(alias string) Opt {
	return optFunc{
		agg: func(b *q.AggregateBuilder) { b.Reduce("COUNT", "", alias) },
	}
}

func Sum(field, alias string) Opt {
	return optFunc{
		agg: func(b *q.AggregateBuilder) { b.Reduce("SUM", field, alias) },
	}
}

func Avg(field, alias string) Opt {
	return optFunc{
		agg: func(b *q.AggregateBuilder) { b.Reduce("AVG", field, alias) },
	}
}
