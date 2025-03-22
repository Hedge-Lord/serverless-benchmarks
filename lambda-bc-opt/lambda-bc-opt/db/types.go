package db

type Op interface{}

type GetOp struct {
	K string `json:"k"`
}

type SetOp struct {
	K string `json:"k"`
	V string `json:"v"`
}
