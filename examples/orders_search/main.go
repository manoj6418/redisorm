// examples/orders_search/main.go
package main

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/redis/go-redis/v9"

	"github.com/manojoshi/redisorm/driver"
	"github.com/manojoshi/redisorm/index"
	q "github.com/manojoshi/redisorm/query"
	"github.com/manojoshi/redisorm/repository"
)

type Order struct {
	ID        string `redisorm:"@order_id,TAG,SORTABLE"`
	Status    string `redisorm:"@status,TAG"`
	Warehouse int    `redisorm:"@warehouse_id,TAG"`
	Qty       int    `redisorm:"@qty,NUMERIC"`
	PromiseTS int64  `redisorm:"@promise_ts,NUMERIC,SORTABLE"`
	CreatedTS int64  `redisorm:"@created_ts,NUMERIC,SORTABLE"`
}

func main() {
	ctx := context.Background()

	rdb := redis.NewClient(&redis.Options{
		Addr: "localhost:6379",
	})

	conn := driver.NewRedisearchConn(rdb)
	defer conn.Close()

	if err := index.AutoCreate(ctx, conn, Order{},
		index.WithName("order_idx"),
		index.WithPrefixes("order:"), // hashes like order:123
	); err != nil {
		log.Fatalf("index create: %v", err)
	}

	seed(ctx, rdb)

	repo := repository.New("order_idx", conn)

	orders, err := repo.Search(
		ctx,
		q.MatchAll(),
		repository.Select("order_id", "qty", "promise_ts"),
		repository.SortAsc("promise_ts"),
		repository.Limit(0, 1),
	)
	if err != nil {
		log.Fatalf("search err: %v", err)
	}

	for _, a := range orders {
		fmt.Printf("a %+v %+v\n", a, len(orders))
	}
}

// seed writes two demo orders into Redis hashes.
func seed(ctx context.Context, rdb *redis.Client) {
	hset := func(key string, m map[string]any) {
		if err := rdb.HSet(ctx, key, m).Err(); err != nil {
			log.Fatal(err)
		}
	}
	now := time.Now().Unix()

	hset("order:101", map[string]any{
		"order_id":     "101",
		"status":       "PENDING",
		"warehouse_id": 1,
		"qty":          2,
		"promise_ts":   now + 3600,
		"created_ts":   now,
	})
	hset("order:102", map[string]any{
		"order_id":     "102",
		"status":       "PENDING",
		"warehouse_id": 2,
		"qty":          1,
		"promise_ts":   now + 7200,
		"created_ts":   now,
	})
}
