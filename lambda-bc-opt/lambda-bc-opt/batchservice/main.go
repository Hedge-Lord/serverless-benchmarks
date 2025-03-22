package main

import (
	"fmt"
	"log/slog"
	"encoding/json"
	"os"
	"strconv"

	"lambda-bc-opt/db"
	"lambda-bc-opt/utility"

	"github.com/valyala/fasthttp"
)

var rdb db.KeyValueStoreDB

func getHandler(ctx *fasthttp.RequestCtx) {
	body := ctx.Request.Body()

	var getOp db.GetOp

	err := json.Unmarshal(body, &getOp)
	if err != nil {
		slog.Error(fmt.Sprintf("Unmarshal error => %v", err))
		ctx.Error("Invalid JSON payload", fasthttp.StatusBadRequest)
		return
	}

	result, err := rdb.Get(getOp.K)
	if err != nil {
		slog.Error(fmt.Sprintf("DB Access error => %v", err))
		ctx.Error("Expected HTTP request, not HTTPS", fasthttp.StatusBadRequest)
		return
	}
	slog.Debug(fmt.Sprintf("%s value in DB => %s\n", getOp.K, result))

	ctx.SetBodyString(result)
}

func main() {
	// Check if the batch size argument is provided
	if len(os.Args) < 2 {
		slog.Error("Batch size argument is required")
		os.Exit(1)
	}

	// Parse the batch size argument
	batchSize, err := strconv.Atoi(os.Args[1])
	if err != nil {
		slog.Error(fmt.Sprintf("Invalid batch size: %v", err))
		os.Exit(1)
	}

	opts := &slog.HandlerOptions{
		Level: slog.LevelInfo,
		// Level: slog.LevelDebug,
	}
	handler := slog.NewTextHandler(os.Stdout, opts)
	logger := slog.New(handler)
	slog.SetDefault(logger)

	// DB
	redisHost := utility.GetEnv("REDIS_HOST", "-1")
	redisPort := utility.GetEnv("REDIS_PORT", "-1")
	slog.Info(fmt.Sprintf("redisHost redisPort => %s %s", redisHost, redisPort))

	// API
	host := utility.GetEnv("APP_HOST", "-1")
	port := utility.GetEnv("APP_PORT", "-1")
	address := fmt.Sprintf("%s:%s", host, port)

	redisPassword := os.Getenv("DB_PASS")
	rdb = db.ConsBatchedRedisDB(redisHost, redisPort, redisPassword, 1, batchSize)
	// rdb = db.ConsMockRedisDB()
	slog.Info(fmt.Sprintf("batchSize => %d", batchSize))

	fmt.Printf("Server listening onnn %s\n", address)
	server := &fasthttp.Server{
		Handler:      getHandler,
	}
	if err := server.ListenAndServe(address); err != nil {
		slog.Error(fmt.Sprintf("Error in ListenAndServe: %v", err))
	}
}
