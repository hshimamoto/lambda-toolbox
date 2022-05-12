// MIT License Copyright (C) 2022 Hiroshi Shimamoto
package main

import (
	"context"

	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/ecs"
	"github.com/aws/aws-sdk-go-v2/service/ecs/types"
)

type ECSClient struct {
	client *ecs.Client
}

func NewECSClient() (*ECSClient, error) {
	cfg, err := config.LoadDefaultConfig(context.TODO())
	if err != nil {
		return nil, err
	}
	client := &ECSClient{
		client: ecs.NewFromConfig(cfg),
	}
	return client, nil
}

func (cli *ECSClient) DescribeClusters(arns []string) ([]types.Cluster, error) {
	input := &ecs.DescribeClustersInput{
		Clusters: arns,
	}
	output, err := cli.client.DescribeClusters(context.TODO(), input)
	if err != nil {
		return nil, err
	}
	return output.Clusters, nil
}

func (cli *ECSClient) ListClusters() ([]string, error) {
	input := &ecs.ListClustersInput{}
	output, err := cli.client.ListClusters(context.TODO(), input)
	if err != nil {
		return nil, err
	}
	return output.ClusterArns, nil
}

func (cli *ECSClient) DescribeTaskDefinition(arn string) (*types.TaskDefinition, error) {
	input := &ecs.DescribeTaskDefinitionInput{
		TaskDefinition: &arn,
	}
	output, err := cli.client.DescribeTaskDefinition(context.TODO(), input)
	if err != nil {
		return nil, err
	}
	return output.TaskDefinition, nil
}

func (cli *ECSClient) ListTaskDefinitions() ([]string, error) {
	input := &ecs.ListTaskDefinitionsInput{}
	output, err := cli.client.ListTaskDefinitions(context.TODO(), input)
	if err != nil {
		return nil, err
	}
	return output.TaskDefinitionArns, nil
}
