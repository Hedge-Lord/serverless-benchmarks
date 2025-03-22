package main

import (
	"context"
	"fmt"
	"lambda-bc-opt/db"
	"log/slog"
	"os"
	"time"
	"sync"

	"github.com/redis/go-redis/v9"
)

func main() {
	opts := &slog.HandlerOptions{
		// Level: slog.LevelDebug,
		Level: slog.LevelInfo,
	}
	handler := slog.NewTextHandler(os.Stdout, opts)
	logger := slog.New(handler)
	slog.SetDefault(logger)

	// db := db.ConsRedisDB("localhost", "6379")
	db1 := redis.NewClient(&redis.Options{
		Addr:     fmt.Sprintf("%s:%s", "localhost", "6379"),
		DB:       0,
		PoolSize: 1,
	})
	db2 := db.ConsRedisDB("localhost", "6379", "", 1)
	db3 := db.ConsBatchedRedisDB("localhost", "6379", "", 1, 100)

	ctx := context.Background()
	result, _ := db1.Get(ctx, "key").Result()
	slog.Debug(result)

	for {
		n := 100000
		//
		t1 := time.Now()
		for i := 0; i < n; i++ {
			db1.Get(ctx, "key").Result()
		}
		t2 := time.Now()
		slog.Info(fmt.Sprintf("Seq => %v", t2.Sub(t1)))
		time.Sleep(3 * time.Second)
		//
		t1 = time.Now()
		pipe := db1.Pipeline()
		for i := 0; i < n; i++ {
			pipe.Get(ctx, "key")
		}
		pipe.Exec(ctx)
		t2 = time.Now()
		slog.Info(fmt.Sprintf("Pipe => %v", t2.Sub(t1)))
		time.Sleep(3 * time.Second)
		// with naive (db2)
		var wg sync.WaitGroup

		t1 = time.Now()
		workerCnt := n / 100
		wg.Add(workerCnt)
		for i := 0; i < workerCnt; i++ {
			go func () {
				for t := 0; t < n / workerCnt; t++ {
					db2.Get("key")
				}
				wg.Done()
			} ()
		}
		wg.Wait()
		t2 = time.Now()
		slog.Info(fmt.Sprintf("naive => %v", t2.Sub(t1)))
		time.Sleep(3 * time.Second)
		// with batched (db3)
		t1 = time.Now()
		workerCnt = n / 100
		wg.Add(workerCnt)
		for i := 0; i < workerCnt; i++ {
			go func () {
				for t := 0; t < n / workerCnt; t++ {
					db3.Get("key")
				}
				wg.Done()
			} ()
		}
		wg.Wait()
		t2 = time.Now()
		slog.Info(fmt.Sprintf("batched => %v", t2.Sub(t1)))
		time.Sleep(3 * time.Second)

	}
}
