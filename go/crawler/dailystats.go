package crawler

import (
	"fmt"
	"time"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/dynamodb"
	"github.com/samsarahq/go/oops"
)

func timeToMs(t time.Time) int64 {
	return t.UnixNano() / 1e6
}

func UpdateDailyStats(aptInfos []*AptInfo) error {
	svc := dynamodb.New(session.New(&aws.Config{
		Region: aws.String("us-west-2")},
	))

	writeRequests := make([]*dynamodb.WriteRequest, 0)
	for i, aptInfo := range aptInfos {
		request := &dynamodb.WriteRequest{
			PutRequest: &dynamodb.PutRequest{
				Item: map[string]*dynamodb.AttributeValue{
					"AptNum": &dynamodb.AttributeValue{
						S: aws.String(aptInfo.AptNum),
					},
					"CreatedAtMs": &dynamodb.AttributeValue{
						N: aws.String(fmt.Sprintf("%d", timeToMs(aptInfo.CreatedAt))),
					},
					"Bedroom": &dynamodb.AttributeValue{
						S: aws.String(aptInfo.Bedroom),
					},
					"Bath": &dynamodb.AttributeValue{
						S: aws.String(aptInfo.Bath),
					},
					"Sqft": &dynamodb.AttributeValue{
						N: aws.String(fmt.Sprintf("%d", aptInfo.Sqft)),
					},
					"Price": &dynamodb.AttributeValue{
						N: aws.String(fmt.Sprintf("%d", aptInfo.Price)),
					},
					"AvailableStart": &dynamodb.AttributeValue{
						S: aws.String(aptInfo.AvailableStart.String()),
					},
					"AvailableEnd": &dynamodb.AttributeValue{
						S: aws.String(aptInfo.AvailableEnd.String()),
					},
					"Signature": &dynamodb.AttributeValue{
						S: aws.String(aptInfo.Signature),
					},
				},
			},
		}
		writeRequests = append(writeRequests, request)

		// Dynamo has a max req len of 25.
		if len(writeRequests) == 25 || i == len(aptInfos)-1 {
			_, err := svc.BatchWriteItem(&dynamodb.BatchWriteItemInput{
				RequestItems: map[string][]*dynamodb.WriteRequest{
					"AvalonDailyStats": writeRequests,
				},
			})
			if err != nil {
				return oops.Wrapf(err, "fail to batch write daily stats")
			}
			//log.Printf("upload, i = %d, len = %d", i, len(writeRequests))
			writeRequests = make([]*dynamodb.WriteRequest, 0)
		}
	}

	return nil
}
