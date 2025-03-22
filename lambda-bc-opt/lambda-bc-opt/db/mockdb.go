package db

// import "time"

type MockRedisDB struct{}

// KeyValueStoreDB
func (rdb *MockRedisDB) Get(k string) (string, error) {
	// time.Sleep(20 * time.Second)
	result := "0"
	return result, nil
}
func (rdb *MockRedisDB) Set(k string, v string) error {
	return nil
}

func ConsMockRedisDB() *MockRedisDB {
	return &MockRedisDB{}
}
