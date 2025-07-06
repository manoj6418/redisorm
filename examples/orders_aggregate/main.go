package main

import (
	"context"
	"fmt"
	"log"
	"math/rand"
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

// ---------------------------------------------------------------------
// Main
// ---------------------------------------------------------------------

func main() {
	ctx := context.Background()

	rdb := redis.NewClient(&redis.Options{Addr: "localhost:6379"})
	conn := driver.NewRedisearchConn(rdb)
	defer conn.Close()

	if err := index.AutoCreate(ctx, conn, Order{},
		index.WithName("order_idx"),
		index.WithPrefixes("order:"),
	); err != nil {
		log.Fatalf("create index: %v", err)
	}

	if err := seedOrders(ctx, rdb, 100); err != nil {
		log.Fatalf("seeding: %v", err)
	}

	repo := repository.New("order_idx", conn)

	results, err := repo.Aggregate(
		ctx,
		q.MatchAll(), // no filter
		repository.Group(q.By("warehouse_id"), q.By("status")),
		repository.Count("orders"),
		repository.Sum("qty", "total_qty"),
		repository.Avg("qty", "avg_qty"),
		repository.Limit(0, 150),
	)
	if err != nil {
		log.Fatalf("aggregate: %v", err)
	}

	fmt.Println(results)

	fmt.Println("Warehouse stats")
	for _, row := range results {
		fmt.Printf("WH %s, status %s → orders=%s  total_qty=%s  avg_qty=%s\n",
			row["warehouse_id"], row["status"], row["orders"], row["total_qty"], row["avg_qty"])
	}
}

func seedOrders(ctx context.Context, rdb *redis.Client, n int) error {
	statuses := []string{"PENDING", "SHIPPED", "CANCELLED"}
	now := time.Now().Unix()

	for i := 1; i <= n; i++ {
		id := fmt.Sprintf("%03d", i)
		key := "order:" + id
		if err := rdb.HSet(ctx, key, map[string]any{
			"order_id":     id,
			"status":       statuses[rand.Intn(len(statuses))],
			"warehouse_id": rand.Intn(4) + 1,
			"qty":          rand.Intn(5) + 1,
			"promise_ts":   now + int64(rand.Intn(7*86400)),
			"created_ts":   now - int64(rand.Intn(7*86400)),
		}).Err(); err != nil {
			return err
		}
	}
	return nil
}
