package query

import (
	"context"
	"errors"
	"github.com/manojoshi/redisorm/scan"
	"strconv"
	"strings"

	"github.com/manojoshi/redisorm/driver"
)

// -------------------------------------------------------------------
// SearchBuilder – fluent builder for FT.SEARCH
// -------------------------------------------------------------------

type Dir string

const (
	Asc  Dir = "ASC"
	Desc Dir = "DESC"
)

type SearchBuilder struct {
	idx           string
	where         Expr
	returnFields  []string
	sortField     string
	dir           Dir
	offset, limit int
	withTotal     bool
	executor      driver.Executor
}

// NewSearch starts a builder. Executor must be provided before Run.
func NewSearch(index string) *SearchBuilder {
	return &SearchBuilder{idx: index, limit: 10_000}
}

func (b *SearchBuilder) Where(e Expr) *SearchBuilder { b.where = e; return b }
func (b *SearchBuilder) Select(fs ...string) *SearchBuilder {
	b.returnFields = append([]string{}, fs...)
	return b
}
func (b *SearchBuilder) SortBy(f string, d Dir) *SearchBuilder {
	b.sortField, b.dir = f, d
	return b
}
func (b *SearchBuilder) Limit(off, lim int) *SearchBuilder {
	b.offset, b.limit = off, lim
	return b
}
func (b *SearchBuilder) WithTotal() *SearchBuilder { b.withTotal = true; return b }
func (b *SearchBuilder) Using(ex driver.Executor) *SearchBuilder {
	b.executor = ex
	return b
}

// RawArgs gives you the complete arg slice for logging / pipeline use.
func (b *SearchBuilder) RawArgs() ([]interface{}, error) {
	var q string
	if b.where == nil || b.where == MatchAll() {
		q = "*"
	} else {
		q = "(" + Compile(b.where) + ")"
	}

	args := []interface{}{"FT.SEARCH", b.idx, q}

	if len(b.returnFields) > 0 {
		args = append(args, "RETURN", strconv.Itoa(len(b.returnFields)))
		for _, f := range b.returnFields {
			args = append(args, f)
		}
	}

	if b.sortField != "" {
		args = append(args, "SORTBY", b.sortField, string(b.dir))
	}

	// LIMIT
	args = append(args, "LIMIT", strconv.Itoa(b.offset), strconv.Itoa(b.limit))

	return args, nil
}

// Run executes the command and decodes into []T (struct or map).
func (b *SearchBuilder) Run(ctx context.Context) ([]map[string]string, error) {
	if b.executor == nil {
		return nil, errors.New("query: executor not set (call Using())")
	}
	args, err := b.RawArgs()
	if err != nil {
		return nil, err
	}

	raw, err := b.executor.Do(ctx, args...)
	if err != nil {
		return nil, err
	}

	return scan.DecodeMaps(raw)
}

// -------------------------------------------------------------------
// AggregateBuilder – fluent builder for FT.AGGREGATE
// -------------------------------------------------------------------

type AggregateBuilder struct {
	idx           string
	where         Expr
	groups        []GroupKey
	reducers      []reducer
	offset, limit int
	executor      driver.Executor
}

type reducer struct{ fn, field, alias string }

func NewAggregate(index string) *AggregateBuilder {
	return &AggregateBuilder{idx: index, limit: 10_000}
}

func (b *AggregateBuilder) Where(e Expr) *AggregateBuilder { b.where = e; return b }
func (b *AggregateBuilder) GroupBy(keys ...GroupKey) *AggregateBuilder {
	b.groups = keys
	return b
}
func (b *AggregateBuilder) Reduce(fn, field, as string) *AggregateBuilder {
	b.reducers = append(b.reducers, reducer{fn, field, as})
	return b
}
func (b *AggregateBuilder) Limit(off, lim int) *AggregateBuilder {
	b.offset, b.limit = off, lim
	return b
}
func (b *AggregateBuilder) Using(ex driver.Executor) *AggregateBuilder {
	b.executor = ex
	return b
}

func (b *AggregateBuilder) RawArgs() ([]interface{}, error) {
	var q string
	if b.where == nil || b.where == MatchAll() {
		q = "*"
	} else {
		q = "(" + Compile(b.where) + ")"
	}

	args := []interface{}{"FT.AGGREGATE", b.idx, q}

	args = append(args, "GROUPBY", strconv.Itoa(len(b.groups)))
	for _, g := range b.groups {
		args = append(args, g.raw)
	}

	for _, r := range b.reducers {
		if strings.EqualFold(r.fn, "COUNT") {
			args = append(args, "REDUCE", r.fn, "0", "AS", r.alias)
			continue
		}
		args = append(args, "REDUCE", r.fn, "1", "@"+r.field, "AS", r.alias)
	}

	args = append(args, "LIMIT", strconv.Itoa(b.offset), strconv.Itoa(b.limit))

	return args, nil
}

func (b *AggregateBuilder) Run(ctx context.Context) ([]map[string]string, error) {
	if b.executor == nil {
		return nil, errors.New("query: executor not set (call Using())")
	}
	args, err := b.RawArgs()
	if err != nil {
		return nil, err
	}

	raw, err := b.executor.Do(ctx, args...)
	if err != nil {
		return nil, err
	}
	return scan.DecodeMaps(raw)
}
