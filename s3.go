// MIT License Copyright (C) 2022 Hiroshi Shimamoto
package main

import (
	"bytes"
	"context"
	"fmt"
	"io"

	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/s3"
)

type Bucket struct {
	name   string
	client *s3.Client
}

func NewBucket(name string) (*Bucket, error) {
	if name == "" {
		return nil, fmt.Errorf("empty name")
	}
	cfg, err := config.LoadDefaultConfig(context.TODO())
	if err != nil {
		return nil, err
	}
	bucket := &Bucket{
		name:   name,
		client: s3.NewFromConfig(cfg),
	}
	return bucket, nil
}

func (b *Bucket) Put(key string, body []byte) error {
	input := &s3.PutObjectInput{
		Bucket: &b.name,
		Key:    &key,
		Body:   bytes.NewBuffer(body),
	}
	_, err := b.client.PutObject(context.TODO(), input)
	return err
}

func (b *Bucket) Get(key string) ([]byte, error) {
	input := &s3.GetObjectInput{
		Bucket: &b.name,
		Key:    &key,
	}
	output, err := b.client.GetObject(context.TODO(), input)
	if err != nil {
		return nil, err
	}
	defer output.Body.Close()
	return io.ReadAll(output.Body)
}
