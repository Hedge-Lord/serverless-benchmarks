package db

import (
	"context"
	"fmt"
	"log"

	// "time"

	"github.com/redis/go-redis/v9"
)

func InitRedis(host string, port string, redisPassword string, poolSize int) *redis.Client {
	return redis.NewClient(&redis.Options{
		Addr:     fmt.Sprintf("%s:%s", host, port),
		Password: redisPassword,
		DB:       0,
		PoolSize: poolSize,
	})
}

type RedisDB struct {
	rc *redis.Client
}

func (rdb *RedisDB) Getrc() *redis.Client {
	return rdb.rc
}

// KeyValueStoreDB
func (rdb *RedisDB) Get(k string) (string, error) {
	result, err := rdb.rc.Get(context.Background(), k).Result()
	if err == redis.Nil {
		return "0", nil
	} else if err != nil {
		log.Printf("Error fetching %s from Redis: %v", k, err)
		return "", err
	}
	return result, nil
}
func (rdb *RedisDB) Set(k string, v string) error {
	err := rdb.rc.Set(context.Background(), k, v, 0).Err()
	if err != nil {
		log.Printf("Error updating %s in Redis: %v", k, err)
		return err
	}
	return nil
}

func ConsRedisDB(host string, port string, redisPassword string, poolSize int) *RedisDB {
	rc := InitRedis(host, port, redisPassword, poolSize)
	return &RedisDB{rc: rc}
}
