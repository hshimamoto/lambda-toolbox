// MIT License Copyright (C) 2022 Hiroshi Shimamoto
package main

import (
	"context"
	"fmt"

	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/lambda"
)

func LambdaUpdateFunctionCode(fname, bucket, zipname string) error {
	cfg, err := config.LoadDefaultConfig(context.TODO())
	if err != nil {
		return err
	}
	client := lambda.NewFromConfig(cfg)
	input := &lambda.UpdateFunctionCodeInput{
		FunctionName: &fname,
		S3Bucket:     &bucket,
		S3Key:        &zipname,
	}
	output, err := client.UpdateFunctionCode(context.TODO(), input)
	if err != nil {
		return err
	}
	fmt.Printf("UpdateFunctionCode: %s\n", *output.FunctionName)
	return nil
}
