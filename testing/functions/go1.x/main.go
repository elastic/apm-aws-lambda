package main

import (
	"fmt"
	"log"

	"github.com/aws/aws-lambda-go/events"
	"github.com/aws/aws-lambda-go/lambda"

	_ "go.elastic.co/apm/module/apmlambda/v2"
)

var coldstart = make(chan struct{})

func Handle(req events.APIGatewayProxyRequest) (events.APIGatewayProxyResponse, error) {
	isColdstart := "true"
	select {
	case <-coldstart:
		isColdstart = "false"
	default:
		close(coldstart)
	}
	log.Println("Example function log", req.RequestContext.RequestID)
	response := events.APIGatewayProxyResponse{
		StatusCode: 200,
		Body:       fmt.Sprintf("Hello world %s!", req.RequestContext.RequestID),
		Headers: map[string]string{
			"coldstart": isColdstart,
		},
	}
	return response, nil
}

func main() {
	lambda.Start(Handle)
}
