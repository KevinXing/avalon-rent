package main

import (
	"context"
	"log"

	"github.com/KevinXing/avalon-rent/go/crawler"
	"github.com/aws/aws-lambda-go/lambda"
)

func main() {
	lambda.Start(HandleRequest)
}

func HandleRequest(ctx context.Context) error {
	aptInfos, err := crawler.CreateAptInfos()
	if err != nil {
		crawler.SendErr("Avalon Apt Info Get Error", err)
		return err
	}
	if err := crawler.UpdateDailyStats(aptInfos); err != nil {
		crawler.SendErr("Avalon Update Daily Stats Error", err)
		return err
	}
	log.Println("Update daily stats success")
	return nil
}
