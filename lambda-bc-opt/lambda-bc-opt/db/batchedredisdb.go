package db

import (
	"context"
	"fmt"
	"log/slog"
	"net"
	"sync"
	"time"

	"github.com/redis/go-redis/v9"
)

type BatchedRedisDB struct {
	rc *redis.Client
	batchSize int
}

type BatchOp struct {
	op Op
	ch chan<- string
}

type BatchResponse struct {
	redisResponse redis.Cmder
	batchOp       BatchOp
}

var batch chan BatchOp
var loopInterval = 100 * time.Microsecond

var mu sync.Mutex
var lastExec time.Time

func execPipeline(ctx context.Context, pipe redis.Pipeliner) error {
	for retries := 0; retries < 3; retries++ {
		_, err := pipe.Exec(ctx)
		if err == nil {
			return nil
		} else if netErr, ok := err.(net.Error); ok && netErr.Timeout() {
			slog.Warn(fmt.Sprintf("Pipeline operation timed out. Retrying... attempt %d", retries+1))
		} else if err != redis.Nil {
			slog.Warn(fmt.Sprintf("Pipeline failed with error: %v. Retrying... attempt %d", err, retries+1))
		}
		time.Sleep(2 * time.Millisecond)
	}
	return fmt.Errorf("pipeline execution failed after 3 retries")
}

func execBatch(rdb *BatchedRedisDB) {
	ctx := context.Background()
	batchResponses := make(chan BatchResponse, rdb.batchSize)
	pipe := rdb.rc.Pipeline()
	processedCount := 0
	for i := 0; i < rdb.batchSize; i++ {
		select {
		case cur_op := <-batch:
			{
				processedCount++
				switch v := cur_op.op.(type) {
				case GetOp:
					result := pipe.Get(ctx, v.K)
					batchResponses <- BatchResponse{
						redisResponse: result,
						batchOp:       cur_op}
				case SetOp:
					result := pipe.Set(ctx, v.K, v.V, 0)
					batchResponses <- BatchResponse{
						redisResponse: result,
						batchOp:       cur_op}
				default:
					slog.Error("Unknown operation type")
				}
			}
		default:
			break
		}
	}
	if processedCount > 0 {
		slog.Debug(fmt.Sprintf(">> processedCount %d", processedCount))
		slog.Debug(fmt.Sprintf(">> batch size %d", len(batch)))
		execPipeline(ctx, pipe)
	}
forLoop:
	for {

		select {
		case response := <-batchResponses:
			redisResponse := response.redisResponse
			batchOp := response.batchOp
			switch batchOp.op.(type) {
			case GetOp:
				if getResp, ok := redisResponse.(*redis.StringCmd); ok {
					batchOp.ch <- getResp.Val()
				} else {
					batchOp.ch <- "error"
				}
			case SetOp:
				if setResp, ok := redisResponse.(*redis.StatusCmd); ok {
					batchOp.ch <- setResp.Val()
				} else {
					batchOp.ch <- "error"
				}
			}
		default:
			break forLoop
		}
	}

	if processedCount > 0 {
		curExec := time.Now()
		diff := curExec.Sub(lastExec)
		slog.Debug(fmt.Sprintf("Time difference: %v", diff))
		lastExec = curExec
	}
}

func appendToBatch(rdb *BatchedRedisDB, op Op, ch chan<- string) {
	switch op.(type) {
	case GetOp:
		batch <- BatchOp{op, ch}
	case SetOp:
		batch <- BatchOp{op, ch}
	default:
		slog.Error("Unknown operation type")
	}
	if len(batch) >= rdb.batchSize {
		execBatch(rdb)
		slog.Debug("exec: BATCH FULL!")
	}
}

func GetBatch() chan BatchOp {
	return batch
}

// KeyValueStoreDB & AKeyVlaueStoreDB
func (rdb *BatchedRedisDB) Get(k string) (string, error) {
	op := GetOp{K: k}
	ch := make(chan string)
	go func() {
		appendToBatch(rdb, op, ch)
	}()
	result := <-ch

	return result, nil
}
func (rdb *BatchedRedisDB) AGet(k string, cb chan<- string) error {
	op := GetOp{K: k}
	go func() {
		appendToBatch(rdb, op, cb)
	}()
	return nil
}
func (rdb *BatchedRedisDB) Set(k string, v string) error {
	op := SetOp{K: k, V: v}
	ch := make(chan string)
	go func() {
		appendToBatch(rdb, op, ch)
	}()
	<-ch
	return nil
}

func ConsBatchedRedisDB(host string, port string, redisPassword string, poolSize int, batchSize int) *BatchedRedisDB {
	rc := InitRedis(host, port, redisPassword, poolSize)
	rdb := BatchedRedisDB{rc: rc, batchSize: batchSize}
	batch = make(chan BatchOp, 100*batchSize)

	go func(rdb *BatchedRedisDB) {
		for range time.Tick(loopInterval) {
			if len(batch) > 0 {
				slog.Debug(fmt.Sprintf("exec: TL reached! batch size : %d", len(batch)))
				execBatch(rdb)
			}
		}
	}(&rdb)

	return &rdb
}
