package main

import (
	"github.com/aws/aws-lambda-go/cfn"
	"github.com/aws/aws-lambda-go/lambda"
	"github.com/pkg/errors"
	"context"
)

func main() {
	lambda.Start(cfn.LambdaWrap(processEvent))
}

type ManualAppProperties struct {
	DomainName string
	ListenerArn string
}

func processEvent(_ context.Context, event cfn.Event) (string, map[string]interface{}, error) {
	properties, err := logGroupProperties(event.ResourceProperties)
	if err != nil {
		return "", nil, err
	}
	logs, err := logService(properties)
	if err != nil {
		return "", nil, err
	}
	switch event.RequestType {
	case cfn.RequestDelete:
		if !hc.IsFailurePhysicalResourceId(event.PhysicalResourceID) {
			_, err := logs.DeleteLogGroupRequest(&.DeleteLogGroupInput{
				LogGroupName: &event.PhysicalResourceID,
			}).Send()
			if err != nil {
				return event.PhysicalResourceID, nil, errors.Wrapf(err, "could not delete log group %s", event.PhysicalResourceID)
			}
		}
		return event.PhysicalResourceID, nil, nil
	case cfn.RequestCreate:
		return createLogGroup(logs, properties)
	case cfn.RequestUpdate:
		oldProperties, err := logGroupProperties(event.OldResourceProperties)
		if err != nil {
			return "", nil, err
		}
		if oldProperties.LogGroupName != properties.LogGroupName || !hc.IsSameRegion(event, oldProperties.Region, properties.Region) {
			return createLogGroup(logs, properties)
		}
		data, err := updateLogGroup(logs, event, oldProperties, properties)
		return event.PhysicalResourceID, data, err
	default:
		return event.PhysicalResourceID, nil, errors.Errorf("unknown request type %s", event.RequestType)
	}
}