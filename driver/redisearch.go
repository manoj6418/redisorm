// driver/redisearch.go
//
// Thin shim over github.com/redis/go-redis/v9 that satisfies the
// redisorm.Executor interface and adds a few convenience helpers
// (cursor paging, pipeline batching, OpenTelemetry spans).
//
// Usage:
//
//	import (
//	    "github.com/redis/go-redis/v9"
//	    "github.com/yourorg/redisorm/driver"
//	)
//
//	rdb := redis.NewClient(&redis.Options{Addr: "localhost:6379"})
//	conn := driver.NewRedisearchConn(rdb)
//	repo := redisorm.NewRepository[Order]("order_idx", conn)
//	orders, _ := repo.Search(ctx, redisorm.Eq("status", "PENDING"))
package driver

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/redis/go-redis/v9"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
)

// Executor is re-exported so callers can assert that RedisearchConn
// meets the redisorm.Executor contract without importing the root lib.
type Executor interface {
	Do(ctx context.Context, args ...interface{}) (any, error)
}

// RedisearchConn implements redisorm.Executor on top of *redis.Client.
type RedisearchConn struct {
	client *redis.Client
}

// NewRedisearchConn wraps an existing go-redis client.
func NewRedisearchConn(c *redis.Client) *RedisearchConn { return &RedisearchConn{client: c} }

// Do satisfies the redisorm.Executor interface.
func (rc *RedisearchConn) Do(ctx context.Context, args ...interface{}) (any, error) {
	// span for tracing & slow-query logging
	ctx, span := otel.Tracer("redisorm.driver").Start(ctx, "redis.do")
	defer span.End()

	start := time.Now()
	res, err := rc.client.Do(ctx, args...).Result()
	elapsed := time.Since(start)

	span.SetAttributes(
		attribute.String("redis.cmd", stringifyCmd(args)),
		attribute.Float64("redis.duration_ms", float64(elapsed.Milliseconds())),
	)
	if err != nil {
		span.RecordError(err)
	}
	return res, err
}

// Close conveniently closes the underlying *redis.Client.
func (rc *RedisearchConn) Close() error { return rc.client.Close() }

// ----------------------------------------------------------------------------
// Helper APIs – optional but handy
// ----------------------------------------------------------------------------

// CursorRead wraps `FT.CURSOR READ` for streaming huge aggregates.
func (rc *RedisearchConn) CursorRead(
	ctx context.Context, index string, cursor uint64, count int,
) ([][]string, uint64, error) {

	if cursor == 0 {
		return nil, 0, errors.New("driver: cursor id must be > 0")
	}

	args := []interface{}{"FT.CURSOR", "READ", index, cursor, "COUNT", count}
	raw, err := rc.Do(ctx, args...)
	if err != nil {
		return nil, 0, err
	}

	reply, ok := raw.([]interface{})
	if !ok || len(reply) != 2 {
		return nil, 0, errors.New("driver: unexpected CURSOR READ reply shape")
	}

	rowsRaw, newCursor := reply[0].([]interface{}), reply[1].(int64)
	rows := make([][]string, len(rowsRaw))
	for i, r := range rowsRaw {
		vals := r.([]interface{})
		row := make([]string, len(vals))
		for j, v := range vals {
			row[j] = toString(v)
		}
		rows[i] = row
	}
	return rows, uint64(newCursor), nil
}

// Pipeline executes a batch of commands and returns raw results.
// Helpful when you need to issue many FT.SEARCH calls in parallel.
func (rc *RedisearchConn) Pipeline(
	ctx context.Context, cmds [][]interface{},
) ([]any, error) {

	pipe := rc.client.Pipeline()
	results := make([]*redis.Cmd, len(cmds))

	for i, cmd := range cmds {
		results[i] = pipe.Do(ctx, cmd...)
	}
	if _, err := pipe.Exec(ctx); err != nil {
		return nil, err
	}

	out := make([]any, len(results))
	for i, r := range results {
		if err := r.Err(); err != nil {
			out[i] = err
		} else {
			out[i] = r.Val()
		}
	}
	return out, nil
}

// ----------------------------------------------------------------------------
// internal helpers
// ----------------------------------------------------------------------------

func stringifyCmd(args []interface{}) string {
	var sb strings.Builder
	for i, a := range args {
		if i > 0 {
			sb.WriteByte(' ')
		}
		sb.WriteString(toString(a))
	}
	return sb.String()
}

func toString(v interface{}) string {
	switch t := v.(type) {
	case string:
		return t
	case []byte:
		return string(t)
	default:
		return fmt.Sprint(t)
	}
}
