package main

import (
	"io"
	"log"
	"net/http"
	"os"

	"lambda-bc-opt/db"
)

var redisPassword string = os.Getenv("DB_PASS")
var rdb db.KeyValueStoreDB = db.ConsRedisDB("10.10.0.1", "6379", redisPassword, 10)

func getterHandler(w http.ResponseWriter, r *http.Request) {
	rdb.Get("cnt")
}

func main() {
	log.SetOutput(io.Discard)
	http.HandleFunc("/getterNaive", getterHandler)

	log.Println("Starting server on :8080")
	err := http.ListenAndServe(":8080", nil)
	if err != nil {
		log.Fatal("ListenAndServe: ", err)
	}
}
