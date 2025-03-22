package db

import (
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"io"

	"github.com/valyala/fasthttp"
)

type BatchedRedisDBV2 struct {
	batchserviceAddress string
	client *fasthttp.HostClient
}


func (rdb *BatchedRedisDBV2) Get(k string) (string, error) {
	op := GetOp{K: k}
	jsonData, err := json.Marshal(op)
	if err != nil {
		return "", fmt.Errorf("error serializing GetOp: %v", err)
	}
	slog.Debug(fmt.Sprintf("XX %s, %s", k, rdb.batchserviceAddress))
	req := fasthttp.AcquireRequest()
	req.SetHost(rdb.batchserviceAddress)
	req.Header.SetMethod(fasthttp.MethodPost)
	req.SetBody(jsonData)
	resp := fasthttp.AcquireResponse()
	err = rdb.client.Do(req, resp)
	if err != nil {
		panic(err)
	}
	fasthttp.ReleaseRequest(req)

	body := string(resp.Body())
	fasthttp.ReleaseResponse(resp)
	return body, nil
}

func (rdb *BatchedRedisDBV2) Set(k string, v string) error {
	return errors.New("Not implemented!")
}

func ConsBatchedRedisDBV2(batchserviceIP string, batchservicePort string) *BatchedRedisDBV2 {
	address := fmt.Sprintf("%s:%s", batchserviceIP, batchservicePort)
	slog.Debug(fmt.Sprintf("Address is %s", address))
	client := &fasthttp.HostClient{
		Addr: address,
	}
	return &BatchedRedisDBV2{
		batchserviceAddress: address,
		client: client,
	}
}
