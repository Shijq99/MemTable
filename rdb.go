package main

import (
	"github.com/hdt3213/rdb/encoder"
	"github.com/hdt3213/rdb/model"
	"os"
	"time"
)

func rdb() {

	rdbFile, err := os.Create("dump.rdb")
	if err != nil {
		panic(err)
	}
	defer rdbFile.Close()
	enc := encoder.NewEncoder(rdbFile)
	err = enc.WriteHeader()
	if err != nil {
		panic(err)
	}
	auxMap := map[string]string{
		"redis-ver":    "4.0.6",
		"redis-bits":   "64",
		"aof-preamble": "0",
	}
	for k, v := range auxMap {
		err = enc.WriteAux(k, v)
		if err != nil {
			panic(err)
		}
	}

	err = enc.WriteDBHeader(0, 5, 1)
	if err != nil {
		panic(err)
	}
	expirationMs := uint64(time.Now().Add(time.Hour*8).Unix() * 1000)
	err = enc.WriteStringObject("hello", []byte("world"), encoder.WithTTL(expirationMs))
	if err != nil {
		panic(err)
	}
	err = enc.WriteListObject("list", [][]byte{
		[]byte("123"),
		[]byte("abc"),
		[]byte("la la la"),
	}, encoder.WithTTL(expirationMs))
	if err != nil {
		panic(err)
	}
	err = enc.WriteSetObject("set", [][]byte{
		[]byte("123"),
		[]byte("abc"),
		[]byte("la la la"),
	}, encoder.WithTTL(expirationMs))
	if err != nil {
		panic(err)
	}
	err = enc.WriteHashMapObject("list", map[string][]byte{
		"1":  []byte("123"),
		"a":  []byte("abc"),
		"la": []byte("la la la"),
	}, encoder.WithTTL(expirationMs))
	if err != nil {
		panic(err)
	}
	err = enc.WriteZSetObject("list", []*model.ZSetEntry{
		{
			Score:  1.234,
			Member: "a",
		},
		{
			Score:  2.71828,
			Member: "b",
		},
	}, encoder.WithTTL(expirationMs))

	if err != nil {
		panic(err)
	}
	err = enc.WriteEnd()
	if err != nil {
		panic(err)
	}

}
