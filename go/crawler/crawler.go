package crawler

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/s3"
	"github.com/gocolly/colly"
	"github.com/samsarahq/go/oops"
)

const (
	timeLayout     = "Jan 2, 2006"
	alertBucket    = "avalon-alert"
	alertObjectKey = "alert-map"
)

type AptInfo struct {
	AptNum         string
	Url            string
	Bedroom        string
	Bath           string
	Sqft           int
	Price          int
	AvailableStart time.Time
	AvailableEnd   time.Time
	Signature      string
	CreatedAt      time.Time
}

func (a *AptInfo) genAlertKey() string {
	return fmt.Sprintf("%s-%s-%s-%s", a.AptNum, a.Price, a.AvailableStart.String(), a.AvailableEnd.String())
}

type Alert struct {
	MaxPrice      int
	MoveDateStart string
	MoveDateEnd   string
}

func GetPrevAlertMap() map[string]*AptInfo {
	// The session the S3 Downloader will use
	alertMap := make(map[string]*AptInfo)
	sess := session.Must(session.NewSession(&aws.Config{Region: aws.String("us-west-2")}))
	s3Client := s3.New(sess)
	output, err := s3Client.GetObject(&s3.GetObjectInput{
		Bucket: aws.String(alertBucket),
		Key:    aws.String(alertObjectKey),
	})
	if err != nil {
		log.Printf("getObject fail: %v", err)
		return alertMap
	}
	outputBytes, err := ioutil.ReadAll(output.Body)
	if err != nil {
		log.Printf("read output fail: %v", err)
		return alertMap
	}
	defer output.Body.Close()

	if err := json.Unmarshal(outputBytes, &alertMap); err != nil {
		log.Printf("unmarshal json fail, %v", err)
		return alertMap
	}
	return alertMap
}

func UploadAlertMap(alertMap map[string]*AptInfo) {
	sess := session.Must(session.NewSession(&aws.Config{Region: aws.String("us-west-2")}))
	s3Client := s3.New(sess)
	jsonBytes, err := json.Marshal(alertMap)
	if err != nil {
		log.Printf("marshal error: %v", err)
	}
	s3Client.PutObject(&s3.PutObjectInput{
		Body:   aws.ReadSeekCloser(bytes.NewReader(jsonBytes)),
		Bucket: aws.String(alertBucket),
		Key:    aws.String(alertObjectKey),
	})
}

func (a *Alert) FireAlert(aptInfos []*AptInfo) error {
	newAlertInfos := make([]*AptInfo, 0)
	existingAlertInfos := make([]*AptInfo, 0)
	deprecatedAlertInfos := make([]*AptInfo, 0)
	prevAlertMap := GetPrevAlertMap()
	alertMap := make(map[string]*AptInfo)
	for _, aptInfo := range aptInfos {
		if aptInfo.Price > a.MaxPrice {
			continue
		}
		moveDateStart, err := time.Parse(timeLayout, a.MoveDateStart)
		if err != nil {
			return oops.Wrapf(err, "parse move date start %s error", a.MoveDateStart)
		}
		moveDateEnd, err := time.Parse(timeLayout, a.MoveDateEnd)
		if err != nil {
			return oops.Wrapf(err, "parse move date end %s error", a.MoveDateEnd)
		}
		if aptInfo.AvailableStart.After(moveDateEnd) {
			continue
		}
		if aptInfo.AvailableEnd.Before(moveDateStart) {
			continue
		}
		key := aptInfo.genAlertKey()
		if _, ok := prevAlertMap[key]; !ok {
			newAlertInfos = append(newAlertInfos, aptInfo)
			alertMap[key] = aptInfo
		} else {
			existingAlertInfos = append(existingAlertInfos, aptInfo)
			alertMap[key] = aptInfo
			delete(prevAlertMap, key)
		}
	}
	//pretty.Print(newAlertInfos)

	if len(prevAlertMap) > 0 {
		for _, aptInfo := range prevAlertMap {
			deprecatedAlertInfos = append(deprecatedAlertInfos, aptInfo)
		}
	}

	if len(newAlertInfos) == 0 && len(existingAlertInfos) == 0 && len(deprecatedAlertInfos) == 0 {
		log.Println("No result")
		return nil
	}

	if len(newAlertInfos) != 0 || len(deprecatedAlertInfos) != 0 {
		UploadAlertMap(alertMap)
		SendEmail(composeEmail(newAlertInfos, existingAlertInfos, deprecatedAlertInfos))
	} else {
		log.Println("No new result")
	}

	return nil
}

func composeEmail(newAlertInfos []*AptInfo, existingAlertInfos []*AptInfo, deprecatedAlertInfos []*AptInfo) string {
	var content string
	genContent := func(aptInfo *AptInfo) string {
		startDate := aptInfo.AvailableStart.Format("Jan 2")
		endDate := aptInfo.AvailableEnd.Format("Jan 2")
		text := fmt.Sprintf("<p><a href=%s>%s, %s, $%d, %s - %s</p>\n", aptInfo.Url, aptInfo.AptNum, aptInfo.Bedroom, aptInfo.Price, startDate, endDate)
		return text
	}

	content += "<h1>New Results</h1>\n"
	for _, aptInfo := range newAlertInfos {
		content += genContent(aptInfo)
	}

	content += "<h1>Deprecated Results</h1>\n"
	for _, aptInfo := range deprecatedAlertInfos {
		content += genContent(aptInfo)
	}

	content += "<h1>Exisiting Results</h1>\n"
	for _, aptInfo := range existingAlertInfos {
		content += genContent(aptInfo)
	}
	return content
}

func CreateAptInfos() ([]*AptInfo, error) {
	c := colly.NewCollector()
	now := time.Now()
	dateRegex := regexp.MustCompile(`Available (.*) — (.*)`)
	apiInfos := make([]*AptInfo, 0)
	c.OnHTML("li[class=apartment-card]", func(e0 *colly.HTMLElement) {
		aptInfo := &AptInfo{}
		aptInfo.CreatedAt = now
		aptInfo.Url = "https://www.avaloncommunities.com/california/san-francisco-apartments/avalon-at-mission-bay/" + e0.ChildAttr("a", "href")
		e0.ForEach("div[class=content]", func(i int, e *colly.HTMLElement) {
			if strings.Contains(e.Text, "Unavailable") {
				return
			}
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
				aptInfo.AvailableStart = startTime
			}
			if endTime, err := time.Parse(timeLayout, dateMatches[2]+fmt.Sprintf(", %d", now.Year())); err != nil {
				log.Printf("fail to parse available end date for apartment %s, available end date %s, err: %s", aptInfo.AptNum, dateMatches[2], err.Error())
			} else {
				aptInfo.AvailableEnd = endTime
			}
			apiInfos = append(apiInfos, aptInfo)
		})
	})

	c.Visit("https://www.avaloncommunities.com/california/san-francisco-apartments/avalon-at-mission-bay/apartments")
	return apiInfos, nil
}
