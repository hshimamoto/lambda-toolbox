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

func (cli *ECSClient) DescribeTasks(arns []string, cluster string) ([]types.Task, error) {
	input := &ecs.DescribeTasksInput{
		Tasks:   arns,
		Cluster: &cluster,
	}
	output, err := cli.client.DescribeTasks(context.TODO(), input)
	if err != nil {
		return nil, err
	}
	return output.Tasks, nil
}

func (cli *ECSClient) ListTasks(cluster string) ([]string, error) {
	input := &ecs.ListTasksInput{
		Cluster: &cluster,
	}
	output, err := cli.client.ListTasks(context.TODO(), input)
	if err != nil {
		return nil, err
	}
	return output.TaskArns, nil
}

func (cli *ECSClient) RunTask(taskdefp *types.TaskDefinition, spot bool, count int32, name, cluster, subnet string, sgs, cmd []string) ([]types.Task, error) {
	input := &ecs.RunTaskInput{
		TaskDefinition:       taskdefp.TaskDefinitionArn,
		Cluster:              &cluster,
		Count:                &count,
		EnableECSManagedTags: true,
		EnableExecuteCommand: false,
		NetworkConfiguration: &types.NetworkConfiguration{
			AwsvpcConfiguration: &types.AwsVpcConfiguration{
				Subnets:        []string{subnet},
				AssignPublicIp: types.AssignPublicIpEnabled,
				SecurityGroups: sgs,
			},
		},
		Overrides: &types.TaskOverride{
			ContainerOverrides: []types.ContainerOverride{
				types.ContainerOverride{
					Command: cmd,
					Name:    &name,
				},
			},
			TaskRoleArn: taskdefp.TaskRoleArn,
		},
	}
	if spot {
		fargate_spot := "FARGATE_SPOT"
		cps := types.CapacityProviderStrategyItem{
			CapacityProvider: &fargate_spot,
			Weight:           1,
		}
		input.CapacityProviderStrategy = append(input.CapacityProviderStrategy, cps)
	}
	output, err := cli.client.RunTask(context.TODO(), input)
	if err != nil {
		return nil, err
	}
	return output.Tasks, nil
}

func (cli *ECSClient) StopTask(arn, cluster string) (*types.Task, error) {
	input := &ecs.StopTaskInput{
		Task:    &arn,
		Cluster: &cluster,
	}
	output, err := cli.client.StopTask(context.TODO(), input)
	if err != nil {
		return nil, err
	}
	return output.Task, nil
}
