// Package index turns Go structs into RediSearch FT.CREATE statements.
// A single public entry-point, `AutoCreate`, checks whether an index exists
// and creates it if missing.
//
//	type Order struct {
//	    ID          string  `redisorm:"@order_id,PK"`
//	    Status      string  `redisorm:"@status,TAG"`
//	    Qty         int     `redisorm:"@qty,NUMERIC,SORTABLE"`
//	    CreatedAt   int64   `redisorm:"@created_ts,NUMERIC,SORTABLE"`
//	}
//
//	if err := index.AutoCreate(ctx, conn, Order{},
//	    index.WithName("order_idx"),
//	    index.WithPrefixes("order:"),
//	); err != nil {
//	    log.Fatal(err)
//	}
package index

import (
	"context"
	"fmt"
	"reflect"
	"strings"

	"github.com/manojoshi/redisorm/driver"
)

// ------------------------------------------------------------------
// Options
// ------------------------------------------------------------------

type CreateOpt func(*createCfg)

type createCfg struct {
	name      string   // FT index name
	prefixes  []string // HASH/JSON key prefixes
	onJson    bool     // ON JSON (default: HASH)
	stopwords []string
}

func WithName(name string) CreateOpt          { return func(c *createCfg) { c.name = name } }
func WithPrefixes(p ...string) CreateOpt      { return func(c *createCfg) { c.prefixes = p } }
func OnJSON() CreateOpt                       { return func(c *createCfg) { c.onJson = true } }
func WithStopwords(words ...string) CreateOpt { return func(c *createCfg) { c.stopwords = words } }

// ------------------------------------------------------------------
// Public API
// ------------------------------------------------------------------

// AutoCreate builds a schema from the supplied struct model and invokes
// FT.CREATE IF NOT EXISTS.  It is safe to call concurrently – Redis will just
// return an error we ignore when the index already exists.
func AutoCreate(
	ctx context.Context,
	exec driver.Executor,
	model any,
	opts ...CreateOpt,
) error {

	cfg := &createCfg{name: inferIndexName(model)}
	for _, o := range opts {
		o(cfg)
	}

	schemaArgs := BuildSchema(model)
	args := []interface{}{"FT.CREATE", cfg.name}
	if cfg.onJson {
		args = append(args, "ON", "JSON")
	}
	if len(cfg.prefixes) > 0 {
		args = append(args, "PREFIX", len(cfg.prefixes))
		for _, p := range cfg.prefixes {
			args = append(args, p)
		}
	}
	if len(cfg.stopwords) > 0 {
		args = append(args, "STOPWORDS", len(cfg.stopwords))
		for _, s := range cfg.stopwords {
			args = append(args, s)
		}
	}
	args = append(args, "SCHEMA")
	args = append(args, schemaArgs...)

	if _, err := exec.Do(ctx, args...); err != nil &&
		!strings.Contains(err.Error(), "Index already exists") {
		return fmt.Errorf("index: FT.CREATE failed: %w", err)
	}
	return nil
}

// BuildSchema inspects the struct tags (`redisorm:\"@field,TAG,SORTABLE\"`) and
// returns the tail of the SCHEMA clause as []interface{}.
func BuildSchema(model any) []interface{} {
	rt := reflect.TypeOf(model)
	if rt.Kind() == reflect.Pointer {
		rt = rt.Elem()
	}

	var out []interface{}
	for i := 0; i < rt.NumField(); i++ {
		f := rt.Field(i)
		tag := f.Tag.Get("redisorm")
		if tag == "" {
			continue
		}
		parts := strings.Split(tag, ",")
		name := strings.TrimPrefix(parts[0], "@")
		fieldType := "TEXT" // default

		// extra attributes (NUMERIC, TAG, GEO, SORTABLE, PK)
		attrs := parts[1:]
		for _, a := range attrs {
			switch strings.ToUpper(a) {
			case "NUMERIC", "TAG", "GEO", "VECTOR":
				fieldType = strings.ToUpper(a)
			}
		}

		out = append(out, name, fieldType)
		for _, a := range attrs {
			upper := strings.ToUpper(a)
			switch upper {
			case "SORTABLE", "NOINDEX", "NOSTEM":
				out = append(out, upper)
			case "PK":
				out = append(out, "NOINDEX")
			}
		}
	}
	return out
}

// inferIndexName defaults to struct type name snake_cased + \"_idx\".
func inferIndexName(model any) string {
	t := reflect.TypeOf(model)
	if t.Kind() == reflect.Pointer {
		t = t.Elem()
	}
	return snake(t.Name()) + "_idx"
}

// snake converts CamelCase to snake_case.
func snake(s string) string {
	var sb strings.Builder
	for i, r := range s {
		if i > 0 && r >= 'A' && r <= 'Z' {
			sb.WriteByte('_')
		}
		sb.WriteRune(r)
	}
	return strings.ToLower(sb.String())
}
