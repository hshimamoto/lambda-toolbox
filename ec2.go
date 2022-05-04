// MIT License Copyright (C) 2022 Hiroshi Shimamoto
package main

import (
	"context"
	"fmt"

	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
	"github.com/aws/aws-sdk-go-v2/service/ec2/types"
)

type EC2Client struct {
	VpcId  string
	client *ec2.Client
}

func NewEC2Client() (*EC2Client, error) {
	cfg, err := config.LoadDefaultConfig(context.TODO())
	if err != nil {
		return nil, err
	}
	client := &EC2Client{
		client: ec2.NewFromConfig(cfg),
	}
	return client, nil
}

func (cli *EC2Client) DescribeInstances() (string, error) {
	// create filter
	input := &ec2.DescribeInstancesInput{}
	if cli.VpcId != "" {
		fname := "vpc-id"
		filter := types.Filter{
			Name:   &fname,
			Values: []string{cli.VpcId},
		}
		input.Filters = append(input.Filters, filter)
	}
	output, err := cli.client.DescribeInstances(context.TODO(), input)
	if err != nil {
		return "", err
	}
	result := ""
	for _, r := range output.Reservations {
		for _, i := range r.Instances {
			result += fmt.Sprintf("%s\n", *i.InstanceId)
		}
	}
	return result, nil
}
