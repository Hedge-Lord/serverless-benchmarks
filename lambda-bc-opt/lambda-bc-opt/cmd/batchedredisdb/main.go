package main

import (
	"lambda-bc-opt/db"
	"log"
	"runtime"
	"sync"
)

func main() {
	batchedRedisDB := db.ConsBatchedRedisDB("10.10.0.1", "6379", "", 10, 100)

	var wg sync.WaitGroup
	wg.Add(2)

	println(runtime.GOMAXPROCS(0))

	go func() {
		result := batchedRedisDB.Set("g1", "value1")
		log.Printf("result1 %v", result)
		wg.Done()
	}()

	go func() {
		result, _ := batchedRedisDB.Get("g2")
		log.Printf("result2 %s", result)
		wg.Done()
	}()

	wg.Wait()
}
