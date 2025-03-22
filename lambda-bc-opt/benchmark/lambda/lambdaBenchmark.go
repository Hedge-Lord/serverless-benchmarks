package main

import (
	"context"
	"fmt"
	"flag"
	"log/slog"
	"os"
	"sort"
	"sync"
	"time"
	"encoding/json"

	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/lambda"
)

type MyEvent struct {
	NumCalls int `json:"num_calls"`
}

func getEnv(key, defaultValue string) string {
	if value, exists := os.LookupEnv(key); exists {
		return value
	}
	return defaultValue
}

func createLambdaClient() *lambda.Client {
	cfg, err := config.LoadDefaultConfig(context.TODO(), config.WithRegion("us-east-2"))
	if err != nil {
		slog.Error("Unable to load SDK config")
		panic(err)
	}

	return lambda.NewFromConfig(cfg)
}

func invokeLambda(client *lambda.Client, functionName string, numCalls int, wg *sync.WaitGroup, durations *[]time.Duration, mu *sync.Mutex) {
	defer wg.Done()

	slog.Info("Invocation started!")
	startTime := time.Now()

	// Create an instance of MyEvent with the desired NumCalls value
	event := MyEvent{NumCalls: numCalls}

	// Marshal the event into JSON format
	payload, err := json.Marshal(event)
	if err != nil {
		slog.Error(fmt.Sprintf("Failed to marshal event: %v", err))
		return
	}

	output, err := client.Invoke(context.TODO(), &lambda.InvokeInput{
		FunctionName: &functionName,
		Payload:      payload,
	})
	// output, err := client.Invoke(context.TODO(), &lambda.InvokeInput{
	//	FunctionName: &functionName,
	// })
	if err != nil {
		slog.Error(fmt.Sprintf("Failed to invoke Lambda function %s: %v", functionName, err))
		return
	}
	responsePayload := ""
	if output.Payload != nil {
		responsePayload = string(output.Payload)
	}
	slog.Info(fmt.Sprintf("output is => %s", responsePayload))

	executionTime := time.Since(startTime)

	// Safely append the execution time to the durations slice
	mu.Lock()
	*durations = append(*durations, executionTime)
	mu.Unlock()

	slog.Info(fmt.Sprintf("Successfully invoked Lambda function: %s, Execution Time: %v",
		functionName,
		executionTime))
}

func calculatePercentile(durations []time.Duration, percentile float64) time.Duration {
	if len(durations) == 0 {
		return 0
	}

	index := int(float64(len(durations)-1) * percentile / 100)
	return durations[index]
}


func writePercentilesToFile(outputName string, p50, p90, p99 time.Duration) {
	file, err := os.Create(outputName)
	if err != nil {
		slog.Error(fmt.Sprintf("Failed to create output file %s: %v", outputName, err))
		return
	}
	defer file.Close()

	_, err = file.WriteString("Percentile,Execution Time\n")
	if err != nil {
		slog.Error(fmt.Sprintf("Failed to write to output file %s: %v", outputName, err))
		return
	}

	_, err = file.WriteString(fmt.Sprintf("50th,%v\n", p50))
	if err != nil {
		slog.Error(fmt.Sprintf("Failed to write to output file %s: %v", outputName, err))
		return
	}

	_, err = file.WriteString(fmt.Sprintf("90th,%v\n", p90))
	if err != nil {
		slog.Error(fmt.Sprintf("Failed to write to output file %s: %v", outputName, err))
		return
	}

	_, err = file.WriteString(fmt.Sprintf("99th,%v\n", p99))
	if err != nil {
		slog.Error(fmt.Sprintf("Failed to write to output file %s: %v", outputName, err))
		return
	}
}

func main() {
	lambdaClient := createLambdaClient()

	var functionName string
	var outputName string
	var rate int
	var numInvocations int
	var numCalls int
	var logLevel string
	flag.StringVar(&functionName, "functionName", "naive-client", "Name of the Lambda function")
	flag.StringVar(&outputName, "outputName", "result.txt", "Name of the output file")
	flag.IntVar(&rate, "rate", 10, "Rate of invocations per second")
	flag.IntVar(&numInvocations, "numInvocations", 100, "Number of invocations")
	flag.IntVar(&numCalls, "numCalls", 1, "Number of database calls per invocation")
	flag.StringVar(&logLevel, "log", "info", "Log level")
	flag.Parse()
	slog.Info(fmt.Sprintf("Args => %s %s %d %d %d %s", functionName, outputName, rate, numInvocations, numCalls, logLevel))

	// Logging
	var opts *slog.HandlerOptions
	if logLevel == "error" {
		opts = &slog.HandlerOptions{
			Level: slog.LevelWarn,
		}
	} else
	if logLevel == "info" {
		opts = &slog.HandlerOptions{
			Level: slog.LevelInfo,
		}
	}
	handler := slog.NewTextHandler(os.Stdout, opts)
	logger := slog.New(handler)
	slog.SetDefault(logger)

	ticker := time.NewTicker(time.Second / time.Duration(rate))
	defer ticker.Stop()

	var wg sync.WaitGroup

	var durations []time.Duration
	var mu sync.Mutex

	for i := 0; i < numInvocations; i++ {
		<-ticker.C // Wait for the next tick to respect the rate limit

		wg.Add(1)
		go invokeLambda(lambdaClient, functionName, numCalls, &wg, &durations, &mu)
	}

	wg.Wait()


	sort.Slice(durations, func(i, j int) bool {
		return durations[i] < durations[j]
	})

	// Calculate and log the 50th, 90th, and 99th percentiles
	p50 := calculatePercentile(durations, 50)
	p90 := calculatePercentile(durations, 90)
	p99 := calculatePercentile(durations, 99)


	slog.Info(fmt.Sprintf("50th Percentile Execution Time: %v", p50))
	slog.Info(fmt.Sprintf("90th Percentile Execution Time: %v", p90))
	slog.Info(fmt.Sprintf("99th Percentile Execution Time: %v", p99))

	writePercentilesToFile(outputName, p50, p90, p99)

	slog.Info("All Lambda invocations completed.")
}
