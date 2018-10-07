package main

import (
	"encoding/csv"
	"github.com/influxdata/influxdb/client/v2"
	"io/ioutil"
	"log"
	"strconv"
	"strings"
	"time"
)

type FaultType int8

const (
	None    FaultType = 0
	CA      FaultType = 1
	SCB     FaultType = 2
	Unknown FaultType = 3
)

const BatchSize = 200

var countries = map[string]string{
	"STANDARD CHARTERED BANK - MC Credit":   "IN",
	"STANDARD CHARTERED BANK - MC Debit":    "IN",
	"STANDARD CHARTERED BANK - VISA Credit": "IN",
	"STANDARD CHARTERED BANK - VISA Debit":  "IN",
	"STANDARD CHARTERED BANK - MC":          "SG",
	"STANDARD CHARTERED BANK - VISA":        "SG",
	"UAE":                                   "AE",
	"BD":                                    "BD",
	"Bahrain":                               "BH",
	"Brunei":                                "BN",
	"Botswana":                              "BW",
	"Ivoire":                                "CI",
	"Ghana":                                 "GH",
	"Gambia":                                "GM",
	"(HK)":                                  "HK",
	"Indonesia":                             "ID",
	"Jersey":                                "JE",
	"Jordan":                                "JO",
	"Kenya":                                 "KE",
	"Srilanka":                              "LK",
	"Malaysia":                              "MY",
	"MALAYSIA":                              "MY",
	"Nigeria":                               "NE",
	"Nepal":                                 "NP",
	"Pakistan":                              "PK",
	"Thailand":                              "TH",
	"Taiwan":                                "TW",
	"Tanzania":                              "TZ",
	"Uganda":                                "UA",
	"Vietnam":                               "VN",
	"Zambia":                                "ZM",
	"Zimbabwe":                              "ZW",
}

func countryForIssuer(issuer string) string {
	for k, v := range countries {
		if issuer == k {
			return v
		} else if strings.Contains(issuer, k) {
			return v
		}
	}
	return "ZZ"
}

func parseRecord(t []string) (time.Time, map[string]string, map[string]interface{}) {
	amount, err := strconv.ParseFloat(t[13], 64)
	if err != nil {
		amount = 0.0
	}

	timestamp, err := time.Parse("2006-01-02 03:04:05 PM CST", t[24])
	if err != nil {
		timestamp = time.Now()
	}

	status, reason := t[18], t[31]
	var fault FaultType

	switch status {
	case "Successful":
		fault = None
	case "Unavailable":
		fault = SCB
	case "N/A":
		if reason == "Abandoned" || reason == "" {
			fault = None
		} else {
			fault = Unknown
		}
	case "Failed":
		if strings.Contains(reason, "GENERAL_EXCEPTION") {
			fault = CA
		} else {
			fault = None
		}
	}

	tags := map[string]string{
		"country": countryForIssuer(t[0]),
		"fault":   strconv.Itoa(int(fault)),
		// file contains mutliple transaction with same timestamp
		// we add pan to tag in order to ensure transaction get updated
		// propertly
		"pan": t[5],
	}

	fields := map[string]interface{}{
		"cardnum":        t[5],
		"proxypan":       t[6],
		"trans_proxypan": t[7],
		"issuer":         t[0],
		"type":           t[30],
		"reason":         reason,
		"currency":       t[12],
		"amount":         amount,
		"callout_status": t[23],
		"status":         status,
	}

	return timestamp, tags, fields
}

// read CSV file and skip the lines before and including the header record
func readCsv(csvFile string) ([][]string, error) {
	bytes, err := ioutil.ReadFile(csvFile)
	if err != nil {
		return nil, err
	}

	s := string(bytes)
	r := csv.NewReader(strings.NewReader(s[strings.Index(s, "Issuer Name"):]))
	records, err := r.ReadAll()
	if err != nil {
		return nil, err
	}

	return records[1:], err
}

// calculate the first and last index of a slice
func nextBatch(curr, length int) (int, int, bool) {
	if curr < 0 || length <= 0 {
		return 0, 0, true
	}

	first, last, isEnd := curr, curr+BatchSize, false
	if last > length {
		last = length
		isEnd = true
	}

	return first, last, isEnd
}

//
func csvToInfluxDB(csvFile string, url string, database string) error {
	log.Printf("Writing transactions in %s to InfluxDB at %s\n", csvFile, url)

	trans, err := readCsv(csvFile)
	if err != nil {
		return err
	}

	c, err := client.NewHTTPClient(client.HTTPConfig{
		Addr:     url,
		Username: "telegraf",
		Password: "metrics",
	})

	if err != nil {
		return err
	}

	defer c.Close()

	first, last, total := 0, 0, len(trans)
	var isEnd bool
	for {
		bp, _ := client.NewBatchPoints(client.BatchPointsConfig{
			Database:        database,
			RetentionPolicy: "raw",
			Precision:       "s",
		})

		first, last, isEnd = nextBatch(first, total)
		//fmt.Printf(" [ %d : %d ]\n", first , last)
		for _, rec := range trans[first:last] {
			timestamp, tags, fields := parseRecord(rec)
			pt, err := client.NewPoint("transactions", tags, fields, timestamp)
			if err == nil {
				bp.AddPoint(pt)
			}
		}

		if len(bp.Points()) > 0 {
			err = c.Write(bp)
			if err != nil {
				return err
			}
		}

		first = last

		if isEnd {
			break
		}
	}

	log.Printf("Written %d records", total)

	return nil
}
