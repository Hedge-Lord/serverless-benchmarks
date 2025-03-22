package main

import (
	"fmt"
	"lambda-bc-opt/db"
	"log/slog"
	"os"
	"time"
)

func main() {
	opts := &slog.HandlerOptions{
		Level: slog.LevelInfo,
		// Level: slog.LevelDebug,
	}
	handler := slog.NewTextHandler(os.Stdout, opts)
	logger := slog.New(handler)
	slog.SetDefault(logger)

	n := 100000
	counter := 0
	// batchedRedisDB := db.ConsBatchedRedisDBV2("127.0.0.1", "6379")
	// batchedRedisDB := db.ConsBatchedRedisDB("127.0.0.1", "6379", 1)
	batchedRedisDB := db.ConsRedisDB("127.0.0.1", "6379", "", 1)

	key := "cnt"

	start := time.Now()
	for i := 0; i < n; i++ {

		result, err := batchedRedisDB.Get(key)
		if err != nil {
			slog.Warn(err.Error())
		}
		counter++
		// slog.Debug(fmt.Sprintf("counter => %d", counter))
		slog.Debug(fmt.Sprintf("result => %s", result))

	}
	end := time.Now()
	slog.Info(fmt.Sprintf("BatchedRedisDBV2 Get => %v", end.Sub(start)))
}
