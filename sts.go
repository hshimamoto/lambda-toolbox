// MIT License Copyright (C) 2022 Hiroshi Shimamoto
package main

import (
	"context"

	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/sts"
	"github.com/aws/aws-sdk-go-v2/service/sts/types"
)

type STSClient struct {
	InstanceIds []string
	VpcId       *string
	client      *sts.Client
}

func NewSTSClient() (*STSClient, error) {
	cfg, err := config.LoadDefaultConfig(context.TODO())
	if err != nil {
		return nil, err
	}
	client := &STSClient{
		client: sts.NewFromConfig(cfg),
	}
	return client, nil
}

func (cli *STSClient) AssumeRole(arn string) (*types.Credentials, error) {
	session := "session"
	duration := int32(3600)
	input := &sts.AssumeRoleInput{
		RoleArn:         &arn,
		RoleSessionName: &session,
		DurationSeconds: &duration,
	}
	output, err := cli.client.AssumeRole(context.TODO(), input)
	if err != nil {
		return nil, err
	}
	return output.Credentials, nil
}
