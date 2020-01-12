package main

import (
	"fmt"
	"log"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/gocolly/colly"
	"github.com/kr/pretty"
)

const timeLayout = "Jan 2, 2006"

type AptInfo struct {
	AptNum         string
	Bedroom        string
	Bath           string
	Sqft           int
	Price          int
	AvailableStart string
	AvailableEnd   string
	Signature      string
	CreatedAt      string
}

func createAptInfos() ([]*AptInfo, error) {
	c := colly.NewCollector()
	now := time.Now()
	dateRegex := regexp.MustCompile(`Available (.*) — (.*)`)
	apiInfos := make([]*AptInfo, 0)
	c.OnHTML("li[class=apartment-card]", func(e0 *colly.HTMLElement) {
		e0.ForEach("div[class=content]", func(i int, e *colly.HTMLElement) {
			if strings.Contains(e.Text, "Unavailable") {
				return
			}
			aptInfo := &AptInfo{}
			aptInfo.CreatedAt = now.String()
			//fmt.Println(e.ChildAttr("a", "*"))
			//aptInfo.Id = strings.Split(e.ChildAttr("a", "href"), "/")[1]
			aptInfo.Signature = e.ChildText("div[class*=signature]")
			aptInfo.AptNum = e.ChildText("div[class*=title]")

			details := strings.Split(e.ChildText("div[class*=details]"), "•")
			aptInfo.Bedroom = strings.TrimSpace(details[0])
			aptInfo.Bath = strings.TrimSpace(details[1])
			if sqft, err := strconv.Atoi(strings.Split(strings.TrimSpace(details[2]), " ")[0]); err != nil {
				log.Printf("fail to parse sqft for apartment %s, sqft string %s", aptInfo.AptNum, details[2])
			} else {
				aptInfo.Sqft = sqft
			}

			priceStrings := strings.Split(e.ChildText("div[class*=price]"), " ")
			priceString := priceStrings[len(priceStrings)-1]
			priceString = strings.ReplaceAll(priceString[1:], ",", "")
			if price, err := strconv.Atoi(priceString); err != nil {
				log.Printf("fail to parse price for apartment %s, price string %s", aptInfo.AptNum, priceString)
			} else {
				aptInfo.Price = price
			}

			dateMatches := dateRegex.FindStringSubmatch(e.ChildText("div[class*=availability]"))
			//pretty.Print(e.ChildText("div[class*=availability]"))
			if startTime, err := time.Parse(timeLayout, dateMatches[1]+fmt.Sprintf(", %d", now.Year())); err != nil {
				log.Printf("fail to parse available start date for apartment %s, available start date %s, err: %s", aptInfo.AptNum, dateMatches[1], err.Error())
			} else {
				aptInfo.AvailableStart = startTime.String()
			}
			if endTime, err := time.Parse(timeLayout, dateMatches[2]+fmt.Sprintf(", %d", now.Year())); err != nil {
				log.Printf("fail to parse available end date for apartment %s, available end date %s, err: %s", aptInfo.AptNum, dateMatches[2], err.Error())
			} else {
				aptInfo.AvailableEnd = endTime.String()
			}
			apiInfos = append(apiInfos, aptInfo)
		})
	})

	c.Visit("https://www.avaloncommunities.com/california/san-francisco-apartments/avalon-at-mission-bay/apartments")
	return apiInfos, nil
}

func main() {
	infos, _ := createAptInfos()
	pretty.Print(infos)
}
