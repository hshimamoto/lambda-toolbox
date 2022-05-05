// MIT License Copyright (C) 2022 Hiroshi Shimamoto
package main

import (
	"fmt"
	"sort"
	"strings"

	"github.com/aws/aws-sdk-go-v2/service/ec2/types"
)

func EC2InstanceString(i types.Instance) string {
	name := ""
	pubip := ""
	if i.PublicIpAddress != nil {
		pubip = *i.PublicIpAddress
	}
	tags := map[string]string{}
	keys := []string{}
	for _, t := range i.Tags {
		if *t.Key == "Name" {
			name = *t.Value
			continue
		}
		tags[*t.Key] = *t.Value
		keys = append(keys, *t.Key)
	}
	sort.Slice(keys, func(a, b int) bool {
		return keys[a] < keys[b]
	})
	vals := []string{}
	for _, k := range keys {
		vals = append(vals, fmt.Sprintf("%s:%s", k, tags[k]))
	}
	return fmt.Sprintf("%s:%s:%s:%s:%s:[%s]",
		*i.InstanceId, name, i.InstanceType, i.State.Name, pubip,
		strings.Join(vals, ","))
}
