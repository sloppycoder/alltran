package main

import (
	"encoding/csv"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"strings"
)

/*
convert transaction from CSV to json format that can be processed by Telegraf

sample telegraf.conf

*/

type Transaction struct {
	CardNumber    string
	ProxyPan      string
	Country       string
	Issuer        string
	Type          string
	Reason        string
	Currency      string
	Amount        float32
	CallOutStatus string
}

func writeJson(input string, output string) error {

	b, err := ioutil.ReadFile(input)
	if err != nil {
		return err
	}

	s := string(b)
	r := csv.NewReader(strings.NewReader(s[strings.Index(s, "Issuer Name"):]))
	trans, err := r.ReadAll()
	if err != nil {
		return err
	}

	for n, t := range trans[:5] {
		// throw away the header row
		if n == 0 {
			continue
		}

		tran := Transaction{
			t[0],
			t[1],
			t[2],
			t[4],
			t[5],
			t[6],
			t[8],
			100.02,
			t[11],
		}

		blob, _ := json.Marshal(&tran)
		fmt.Println(string(blob))
	}

	return nil
}
