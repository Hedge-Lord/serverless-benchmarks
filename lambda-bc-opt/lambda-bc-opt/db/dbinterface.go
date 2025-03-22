package db

type KeyValueStoreDB interface {
	Get(k string) (string, error)
	Set(k string, v string) error
}

type AKeyValueStoreDB interface {
	AGet(k string, cb chan<- string) error
}
