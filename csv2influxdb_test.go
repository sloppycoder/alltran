package main

import (
	"log"
	"testing"
)

func TestCsv2Json(t *testing.T) {
	err := csvToInfluxDB("AllTransactions_0824181628_0824181728.csv", "http://10.23.218.219:8086", "tds")
	if err != nil {
		log.Fatal(err)
	}
}
