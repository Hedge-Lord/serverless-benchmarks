package main

import (
	"fmt"
	"log/slog"
	"net/http"
	"os"

	"lambda-bc-opt/db"
)

type Op struct {
	opType string
	callback chan struct{}
}

const workerCount int = 100
// const workerCount int = 1
const bufferSize int = 1000000

var tasksChan chan Op = make(chan Op, bufferSize) // Channel of operations, which will be assigned to one of the workers.
var rdbArray [workerCount]db.KeyValueStoreDB      // Each worker has its own db connection.

func startWorkers(dbCallCnt int) {
	// start work
	workerFn := func(workerId int) {
		for {
			select {
			case op := <-tasksChan: // a task is assigned
				slog.Debug(fmt.Sprintf("opType <%s> - workerId %d : Starting", op.opType, workerId))
				var result string = ""
				for i := 0; i < dbCallCnt; i++ {
					result, _ = rdbArray[workerId].Get("cnt")
				}
				op.callback <- struct{}{}
				slog.Debug(fmt.Sprintf("opType <%s> - workerId %d : Ended - %s", op.opType, workerId, result))
			}
			slog.Debug("recurse")
		}
	}
	for i := 0; i < workerCount; i++ {
		go workerFn(i)
	}
}

func main() {
	// Logging setup
	opts := &slog.HandlerOptions{
		Level: slog.LevelInfo,
		// Level: slog.LevelDebug,
	}
	handler := slog.NewTextHandler(os.Stdout, opts)
	logger := slog.New(handler)
	slog.SetDefault(logger)
	//

	slog.Info("Starting benchmark")
	// 1: Start workers
	go func() {
		startWorkers(5)
	} ()
	// the httpHandler will create a task.
	// The task will be declared done when the worker invokes the callback of task
	httpHandler := func(w http.ResponseWriter, r *http.Request) {
		cb := make(chan struct{})
		tasksChan <- Op{
			opType: "Get",
			callback: cb,
		}
		<-cb
	}

	// 2: Create Worker's db connection
	// ddd := db.ConsBatchedRedisDB("localhost", "6379", 1)
	for i := 0; i < workerCount; i++ {
		// rdbArray[i] = db.ConsMockRedisDB()
		// rdbArray[i] = ddd
		rdbArray[i] = db.ConsRedisDB("localhost", "6379", "", 1)
		// rdbArray[i] = db.ConsBatchedRedisDBV2("127.0.0.1", "8090")
	}

	// 3: Start endpoint
	http.HandleFunc("/locallambda", httpHandler)
	slog.Info("Starting server on :8080")
	err := http.ListenAndServe(":8080", nil)
	if err != nil {
		slog.Error(fmt.Sprintf("ListenAndServe: %v", err))
	}
}
