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
	"UAE":       "AE",
	"BD":        "BD",
	"Bahrain":   "BH",
	"Brunei":    "BN",
	"Botswana":  "BW",
	"Ivoire":    "CI",
	"Ghana":     "GH",
	"Gambia":    "GM",
	"(HK)":      "HK",
	"Indonesia": "ID",
	"Jersey":    "JE",
	"Jordan":    "JO",
	"Kenya":     "KE",
	"Srilanka":  "LK",
	"Malaysia":  "MY",
	"MALAYSIA":  "MY",
	"Nigeria":   "NE",
	"Nepal":     "NP",
	"Pakistan":  "PK",
	"Thailand":  "TH",
	"Taiwan":    "TW",
	"Tanzania":  "TZ",
	"Uganda":    "UA",
	"Vietnam":   "VN",
	"Zambia":    "ZM",
	"Zimbabwe":  "ZW",
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
		"status":  status,
		"fault":   string(fault),
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
	}

	return timestamp, tags, fields
}
func csvToInfluxDB(csvFile string, url string, database string) error {
	log.Printf("Writing transactions in %s to InfluxDB at %s\n", csvFile, url)

	b, err := ioutil.ReadFile(csvFile)
	if err != nil {
		return err
	}

	s := string(b)
	r := csv.NewReader(strings.NewReader(s[strings.Index(s, "Issuer Name"):]))
	trans, err := r.ReadAll()
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

	var bp client.BatchPoints
	for n, t := range trans {
		// throw away the header row
		if n == 0 {
			continue
		}

		if n%BatchSize == 0 {
			if len(bp.Points()) > 0 {
				err = c.Write(bp)
				if err != nil {
					return err
				}
				bp = nil
			}
		}

		if bp == nil {
			bp, err = client.NewBatchPoints(client.BatchPointsConfig{
				Database:  database,
				Precision: "s",
			})

			if err != nil {
				return err
			}
		}

		timestamp, tags, fields := parseRecord(t)
		pt, err := client.NewPoint("transactions", tags, fields, timestamp)
		if err != nil {
			return err
		}

		bp.AddPoint(pt)
	}

	if bp != nil {
		err = c.Write(bp)
		if err != nil {
			return err
		}
	}

	return nil
}
