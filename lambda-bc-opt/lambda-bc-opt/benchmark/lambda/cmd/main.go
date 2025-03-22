package main

import (
	"context"
	"log"
	"fmt"

	"github.com/redis/go-redis/v9"
	"github.com/aws/aws-lambda-go/lambda"
)

func handleRequest(ctx context.Context) {
	log.Print("P1")
	db := redis.NewClient(&redis.Options{
		Addr:     fmt.Sprintf("%s:%s", "172.31.14.91", "6379"),
		DB:       0,
		PoolSize: 1,
	})
	result, err := db.Get(context.TODO(), "cnt").Result()
	log.Printf("result => %s", result)
	if err != nil {
		log.Printf("err => %v", err)
	}
	log.Print("HELLO, WORLD")
}

func main() {
	lambda.Start(handleRequest)
}
