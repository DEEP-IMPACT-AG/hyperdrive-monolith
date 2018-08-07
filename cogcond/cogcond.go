package main

import (
	"github.com/aws/aws-lambda-go/lambda"
	"strings"
		"log"
	"github.com/aws/aws-sdk-go-v2/aws/external"
		"github.com/pkg/errors"
	"github.com/aws/aws-lambda-go/events"
	"encoding/json"
	"fmt"
	"github.com/aws/aws-sdk-go-v2/service/ssm"
)

// CognitoEventUserPoolsPreSignupResponse contains the response portion of a PreSignup event
type CognitoEventUserPoolsAllResponse struct {
	AutoConfirmUser bool `json:"autoConfirmUser,omitempty"`
	AutoVerifyEmail bool `json:"autoVerifyEmail,omitempty"`
	AutoVerifyPhone bool `json:"autoVerifyPhone,omitempty"`
}

// CognitoEventUserPoolsPreSignupRequest contains the request portion of a PreSignup event
type CognitoEventUserPoolsPreAll struct {
	events.CognitoEventUserPoolsHeader
	Request  events.CognitoEventUserPoolsPreSignupRequest `json:"request"`
	Response CognitoEventUserPoolsAllResponse             `json:"response"`
}

var zero = CognitoEventUserPoolsPreAll{}

func main() {
	cfg, err := external.LoadDefaultAWSConfig()
	if err != nil {
		log.Fatalf("could not get aws config: %+v\n", err)
	}
	ssms := ssm.New(cfg)
	lambda.Start(processEvent(ssms))
}

type Settings struct {
	All     bool     `json:"all"`
	Domains []string `json:"domains"`
	Emails  []string `json:"emails"`
}

func processEvent(ssms *ssm.SSM) func(CognitoEventUserPoolsPreAll) (CognitoEventUserPoolsPreAll, error) {
	return func(event CognitoEventUserPoolsPreAll) (CognitoEventUserPoolsPreAll, error) {
		fmt.Printf("%+v\n", event)
		userPoolId := event.UserPoolID
		clientId := event.CallerContext.ClientID
		email := event.Request.UserAttributes["email"]
		splitted := strings.Split(email, "@")
		if len(splitted) != 2 {
			return zero, errors.Errorf("invalid email: %s", email)
		}
		domain := splitted[1]
		parameterName := "/hyperdrive/cogcong/" + userPoolId + "/" + clientId
		parameter, err := ssms.GetParameterRequest(&ssm.GetParameterInput{
			Name: &parameterName,
		}).Send()
		if err != nil {
			return zero, errors.Wrap(err, "DDB fetching error")
		}
		if parameter.Parameter == nil || parameter.Parameter.Value == nil {
			return zero, errors.Errorf("no configuration for the client %s of user pool %s", clientId, userPoolId)
		}
		var settings Settings
		err = json.Unmarshal([]byte(*parameter.Parameter.Value), &settings)
		if err != nil {
			return zero, errors.Wrapf(err, "invalid settings for the client %s of user pool %s", clientId, userPoolId)
		}
		if !(settings.All || in(settings.Domains, domain) || in(settings.Emails, email)) {
			return zero, errors.New("not authorized.")
		}
		return event, nil
	}
}

func in(strings []string, val string) bool {
	for _, s := range strings {
		if val == s {
			return true
		}
	}
	return false
}
