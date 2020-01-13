package main

import "github.com/KevinXing/avalon-rent/go/crawler"

import "log"

func main() {
	info, err := crawler.CreateAptInfos()
	if err != nil {
		log.Fatalf("create AptInfo fail: %v", err)
	}
	alert := crawler.Alert{
		MaxPrice:      3700,
		MoveDateStart: "Feb 10, 2020",
		MoveDateEnd:   "Feb 23, 2020",
	}
	alert.FireAlert(info)
}
