package db

type DB map[string]int

func NewDB() DB {
	return make(DB)
}
