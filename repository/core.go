package repository

import (
	"context"
	"fmt"
	q "github.com/manojoshi/redisorm/query"
	"github.com/manojoshi/redisorm/scan"
	"reflect"
	"strings"

	"github.com/redis/go-redis/v9"

	"github.com/manojoshi/redisorm/driver"
	"github.com/manojoshi/redisorm/index"
)

// Repo is the single, reusable handle you inject everywhere.
type Repo struct {
	exec driver.Executor // FT.* commands
	raw  *redis.Client   // low-level HSET / DEL etc.  (optional: can be nil)
}

// WithConn constructs a Repo from the two handles.
func WithConn(exec driver.Executor, raw *redis.Client) *Repo {
	return &Repo{exec: exec, raw: raw}
}

/*───────────────────────────────────────────────────────────────
|  Administrative helpers                                        |
└───────────────────────────────────────────────────────────────*/

// EnsureIndex – thin wrapper over index.AutoCreate with index name injected.
func (r *Repo) EnsureIndex(
	ctx context.Context,
	indexName string,
	model any,
	opts ...index.CreateOpt,
) error {
	opts = append(opts, index.WithName(indexName))
	return index.AutoCreate(ctx, r.exec, model, opts...)
}

// DropIndex drops FT index + optionally deletes keys with given prefix(es).
func (r *Repo) DropIndex(ctx context.Context, indexName string, prefixes ...string) error {
	_, _ = r.exec.Do(ctx, "FT.DROPINDEX", indexName, "DD") // ignore if missing
	if r.raw != nil {
		for _, p := range prefixes {
			iter := r.raw.Scan(ctx, 0, p+"*", 0).Iterator()
			for iter.Next(ctx) {
				_ = r.raw.Del(ctx, iter.Val()).Err()
			}
		}
	}
	return nil
}

/*───────────────────────────────────────────────────────────────
|  Data-loading helpers                                          |
└───────────────────────────────────────────────────────────────*/

// LoadHash inserts one record into a HASH (field tags drive column names).
func (r *Repo) LoadHash(ctx context.Context, key string, record any) error {
	if r.raw == nil {
		return fmt.Errorf("repository: raw Redis client not configured")
	}
	vals := structToMap(record)
	return r.raw.HSet(ctx, key, vals).Err()
}

// LoadBulk writes many records; prefix is used if keyFn returns only ID.
func (r *Repo) LoadBulk(
	ctx context.Context,
	indexName string,
	prefix string,
	records []any,
	keyFn func(any) string,
) error {
	for _, rec := range records {
		key := keyFn(rec)
		if !strings.HasPrefix(key, prefix) {
			key = prefix + key
		}
		if err := r.LoadHash(ctx, key, rec); err != nil {
			return err
		}
	}
	return nil
}

// Generic Search / Aggregate
// Search and Aggregate are generic methods that work with any model type.

// Search over any model
func (r *Repo) Search(
	ctx context.Context,
	indexName string,
	where q.Expr,
	opts ...Opt,
) ([]any, error) {
	sb := q.NewSearch(indexName).Using(r.exec)
	if where != nil {
		sb.Where(where)
	}
	for _, o := range opts {
		o.applySearch(sb)
	}
	raw, err := sb.RawArgs()
	if err != nil {
		return nil, err
	}
	resp, err := r.exec.Do(ctx, raw...)
	if err != nil {
		return nil, err
	}
	return scan.DecodeSlice[any](resp)
}

func (r *Repo) Aggregate(
	ctx context.Context,
	indexName string,
	where q.Expr,
	groupBy []q.GroupKey,
	opts ...Opt,
) ([]map[string]string, error) {

	ab := q.NewAggregate(indexName).
		Using(r.exec).
		GroupBy(groupBy...)
	if where != nil {
		ab.Where(where)
	}
	for _, o := range opts {
		o.applyAgg(ab)
	}

	rawArgs, err := ab.RawArgs()
	if err != nil {
		return nil, err
	}
	resp, err := r.exec.Do(ctx, rawArgs...)
	if err != nil {
		return nil, err
	}
	return scan.DecodeMaps(resp)
}

// structToMap converts a struct or map to a map[string]any.
func structToMap(v any) map[string]any {
	rv := reflect.ValueOf(v)
	if rv.Kind() == reflect.Pointer {
		rv = rv.Elem()
	}
	// map[string]any passed straight through
	if rv.Kind() == reflect.Map {
		out := make(map[string]any)
		iter := rv.MapRange()
		for iter.Next() {
			out[fmt.Sprint(iter.Key())] = iter.Value().Interface()
		}
		return out
	}

	// struct: use redisorm tags
	rt := rv.Type()
	out := make(map[string]any, rt.NumField())
	for i := 0; i < rt.NumField(); i++ {
		f := rt.Field(i)
		tag := f.Tag.Get("redisorm")
		if tag == "" {
			continue
		}
		name := strings.TrimPrefix(strings.Split(tag, ",")[0], "@")
		out[name] = rv.Field(i).Interface()
	}
	return out
}
