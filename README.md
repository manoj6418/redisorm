# redisorm

*A lightweight, opinionated ORM-style wrapper around RediSearch (Redis
Stack) for Go.*

---

## ✨ Features

| Capability                           | Notes                                                                                                                                         |
|--------------------------------------|-----------------------------------------------------------------------------------------------------------------------------------------------|
| **Fluent builders**                  | `FT.SEARCH` (`query.SearchBuilder`) and `FT.AGGREGATE` (`query.AggregateBuilder`).                                                            |
| **Unified option DSL (`repository.Opt`)** | Same helper set ( `Select`, `Limit`, `Group`, `Sum` … ) works for both search & aggregate.                                                    |
| **Connection-centric repo**          | Single `repository.Repo` wraps a `driver.Executor` **and** a raw `go-redis` client.                                                           |
| **Admin helpers**                    | `EnsureIndex`, `DropIndex`, `LoadHash`, `Bulk()` for mass inserts.                                                                            |
| **RESP2 + RESP3 decoder**            | Robust `scan.DecodeSlice / DecodeMaps` handles extra-attributes and legacy array replies.                                                     |
| **No generics on builders**          | The public builders are non-generic, so the code compiles on Go 1.18 – 1.24. Type parameters are used only on helper structs where supported. |
| **Pluggable driver**                 | `driver.RedisearchConn` is a thin shim over `go-redis/v9`; swap for redigo by implementing `driver.Executor`.                                 |

---


## 🚀 Quick start

```bash
go get github.com/manojoshi/redisorm@latest
```

## Connect

```bash
rdb  := redis.NewClient(&redis.Options{Addr: "localhost:6379"})
conn := driver.NewRedisearchConn(rdb)
repo := repository.WithConn(conn, rdb)
```