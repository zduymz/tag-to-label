package provider

import (
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/credentials"
	"github.com/aws/aws-sdk-go/aws/credentials/stscreds"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/ec2"
	"github.com/linki/instrumented_http"
	"k8s.io/klog"
	"strings"
)

type Ec2API interface {
	DescribeTags(input *ec2.DescribeTagsInput) (*ec2.DescribeTagsOutput, error)
}
type AWSProvider struct {
	client Ec2API
}

// AWSConfig contains configuration to create a new AWS provider.
type AWSConfig struct {
	Region     string
	AssumeRole string
	APIRetries int

	AWSCredsFile string
}

type Tag struct {
	Key   string
	Value string
}

func NewAWSProvider(awsConfig AWSConfig) (*AWSProvider, error) {
	config := aws.NewConfig().WithMaxRetries(awsConfig.APIRetries).WithRegion(awsConfig.Region)

	// Only use for testing
	if awsConfig.AWSCredsFile != "" {
		klog.Warning("Not use aws credentials when running on production")

		config.WithCredentials(credentials.NewSharedCredentials(awsConfig.AWSCredsFile, "default"))
	}

	config.WithHTTPClient(
		instrumented_http.NewClient(config.HTTPClient, &instrumented_http.Callbacks{
			PathProcessor: func(path string) string {
				parts := strings.Split(path, "/")
				return parts[len(parts)-1]
			},
		}),
	)

	awsSession, err := session.NewSessionWithOptions(session.Options{
		Config:            *config,
		SharedConfigState: session.SharedConfigEnable,
	})

	if err != nil {
		return nil, err
	}

	if awsConfig.AssumeRole != "" {
		klog.Infof("Assuming role: %s", awsConfig.AssumeRole)
		awsSession.Config.WithCredentials(stscreds.NewCredentials(awsSession, awsConfig.AssumeRole))
	}

	provider := &AWSProvider{
		client: ec2.New(awsSession),
	}

	return provider, nil
}

func (p *AWSProvider) ListTags(instanceIds []*string) (map[string][]*Tag, error) {
	tags := make(map[string][]*Tag)

	describeTagsInput := &ec2.DescribeTagsInput{
		Filters: []*ec2.Filter{
			{
				Name:   aws.String("resource-id"),
				Values: instanceIds,
			},
		},
	}

	for {
		describeTagsOutput, err := p.client.DescribeTags(describeTagsInput)
		if err != nil {
			return nil, err
		}

		describeTagsInput.NextToken = describeTagsOutput.NextToken

		for _, tag := range describeTagsOutput.Tags {
			key := aws.StringValue(tag.ResourceId)
			tagKey := aws.StringValue(tag.Key)
			tagValue := aws.StringValue(tag.Value)
			if _, exist := tags[aws.StringValue(tag.ResourceId)]; exist {
				tags[key] = append(tags[key], &Tag{Key: tagKey, Value: tagValue})
			} else {
				tags[key] = []*Tag{{Key: tagKey, Value: tagValue}}
			}
		}

		if describeTagsOutput.NextToken == nil {
			break
		}

	}
	return tags, nil
}
