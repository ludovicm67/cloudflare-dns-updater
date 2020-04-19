package main

import (
	"errors"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"time"

	"github.com/cloudflare/cloudflare-go"
)

var api *cloudflare.API
var ipv4Enabled bool = false
var ipv6Enabled bool = false
var zoneID, ipv4RecordID, ipv6RecordID string
var lastIP = make(map[string]string)

var checkInterval time.Duration = 120
var ipv4Endpoint string = "https://ipv4-test.ludovic-muller.fr"
var ipv6Endpoint string = "https://ipv6-test.ludovic-muller.fr"

func initCloudflare() {
	var apiKey, apiEmail, zone, record string
	var err error

	if value, ok := os.LookupEnv("CF_API_KEY"); ok {
		apiKey = value
	} else {
		log.Fatal("no api key, please specify one using CF_API_KEY")
	}

	if value, ok := os.LookupEnv("CF_API_EMAIL"); ok {
		apiEmail = value
	} else {
		log.Fatal("no api email, please specify one using CF_API_EMAIL")
	}

	if value, ok := os.LookupEnv("CF_ZONE"); ok {
		zone = value
	} else {
		log.Fatal("no zone (eg. example.com), please specify one using CF_ZONE")
	}

	if value, ok := os.LookupEnv("CF_RECORD"); ok {
		record = value
	} else {
		log.Fatal("no record (eg. test.example.com), please specify one using CF_RECORD")
	}

	api, err = cloudflare.New(apiKey, apiEmail)
	if err != nil {
		log.Fatal(err)
	}

	zoneID, err = api.ZoneIDByName(zone)
	if err != nil {
		log.Fatal(err)
	}

	ipv4Record := cloudflare.DNSRecord{
		Name: record,
		Type: "A",
	}

	ipv6Record := cloudflare.DNSRecord{
		Name: record,
		Type: "AAAA",
	}

	ipv4Records, err := api.DNSRecords(zoneID, ipv4Record)
	if err == nil && len(ipv4Records) > 0 {
		ipv4RecordID = ipv4Records[0].ID
		ipv4Enabled = true
	}

	ipv6Records, err := api.DNSRecords(zoneID, ipv6Record)
	if err == nil && len(ipv6Records) > 0 {
		ipv6RecordID = ipv6Records[0].ID
		ipv6Enabled = true
	}

	if !ipv4Enabled && !ipv6Enabled {
		log.Fatal("no record found")
	}
}

func getIP(endpoint string) (string, error) {
	resp, err := http.Get(endpoint)
	if err != nil {
		return "", err
	}

	defer resp.Body.Close()

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}

	ip := string(body)

	if ip == "" {
		return "", errors.New("empty IP response")
	}

	return ip, nil
}

func updateRecord(endpoint, recordID string) {
	needUpdate := true

	ip, err := getIP(endpoint)
	if err != nil {
		log.Println(err)
		return
	}

	if oldIP, ok := lastIP[endpoint]; ok {
		if oldIP == ip {
			needUpdate = false
		}
	}
	lastIP[endpoint] = ip

	if err != nil {
		log.Println(err)
		return
	}

	if needUpdate {
		log.Println("updating record using following IP", ip)
		record := cloudflare.DNSRecord{
			Content: ip,
		}
		err := api.UpdateDNSRecord(zoneID, recordID, record)
		if err != nil {
			log.Fatal(err)
		}
	}
}

func updateDNS() {
	log.Println("check if DNS records needs update…")

	if ipv4Enabled {
		updateRecord(ipv4Endpoint, ipv4RecordID)
	}

	if ipv6Enabled {
		updateRecord(ipv6Endpoint, ipv6RecordID)
	}
}

func main() {
	initCloudflare()
	updateDNS()
	ticker := time.NewTicker(checkInterval * time.Second)
	for {
		select {
		case <-ticker.C:
			updateDNS()
		}
	}
}
