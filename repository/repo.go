// Package repository offers a thin, type-safe façade on top of the lower-level
// builders in the query package.  It follows the functional-options pattern so
// callers can keep code terse while still accessing the full power of Redisearch.
//
//	repo := repository.New("order_idx", conn)
//	orders, err := repo.Search(ctx,
//	    q.And(q.Eq("status", "PENDING"), q.In("warehouse_id", 45, 46)),
//	    repository.Select("order_id", "qty"),
//	    repository.SortAsc("promise_ts"),
//	    repository.Limit(0, 1000),
//	)
package repository

import (
	"context"

	"github.com/manojoshi/redisorm/driver"
	q "github.com/manojoshi/redisorm/query"
)

// Repository is generic over the domain model.
type Repository struct {
	index string
	exec  driver.Executor
}

// New constructs a repository bound to a RediSearch index.
func New(index string, exec driver.Executor) *Repository {
	return &Repository{index: index, exec: exec}
}

// -------------------------------------------------------------------
// SEARCH
// -------------------------------------------------------------------

// Search executes a FT.SEARCH using the provided where Expr and any search
// options (Select, SortAsc, Limit, …). It decodes the results directly into map[string]string
func (r *Repository) Search(
	ctx context.Context,
	where q.Expr,
	opts ...Opt,
) ([]map[string]string, error) {

	sb := q.NewSearch(r.index).
		Where(where).
		Using(r.exec)

	for _, opt := range opts {
		opt.applySearch(sb)
	}
	return sb.Run(ctx)
}

// -------------------------------------------------------------------
// AGGREGATE
// -------------------------------------------------------------------

// Aggregate runs FT.AGGREGATE.  Caller supplies group-by fields and optional
// reducers.  Result is a slice of map[string]string for maximum flexibility.
func (r *Repository) Aggregate(
	ctx context.Context,
	where q.Expr,
	opts ...Opt,
) ([]map[string]string, error) {

	ab := q.NewAggregate(r.index).
		Where(where).
		Using(r.exec)

	for _, opt := range opts {
		opt.applyAgg(ab)
	}
	return ab.Run(ctx)
}
