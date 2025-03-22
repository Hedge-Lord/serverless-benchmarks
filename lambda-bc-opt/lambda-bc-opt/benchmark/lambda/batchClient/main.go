package main

import (
	"context"
	"log/slog"
	"fmt"
	"os"

	"lambda-bc-opt/db"

	"github.com/aws/aws-lambda-go/lambda"
)

type MyEvent struct {
	NumCalls int `json:"num_calls"`
}

var dbConn db.KeyValueStoreDB = db.ConsBatchedRedisDBV2("172.31.13.83", "8090")

func handleRequest(ctx context.Context, event MyEvent) (string, error) {
	slog.Info("P1")

	var result string
	var err error
	// Loop to call the database the specified number of times
	for i := 0; i < event.NumCalls; i++ {
		result, err = dbConn.Get("cnt")
		if err != nil {
			slog.Error(fmt.Sprintf("error => %v", err))
			break
		}
	}

	slog.Info(fmt.Sprintf("result => %s", result))
	return result, nil
}

func main() {
	opts := &slog.HandlerOptions{
		Level: slog.LevelWarn,
		// Level: slog.LevelInfo,
	}
	handler := slog.NewTextHandler(os.Stdout, opts)
	logger := slog.New(handler)
	slog.SetDefault(logger)

	lambda.Start(handleRequest)
}
