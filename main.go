// MIT License Copyright (C) 2022 Hiroshi Shimamoto
package main

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/aws/aws-lambda-go/events"
	"github.com/aws/aws-lambda-go/lambda"
)

type PostRequest struct {
	Command     string   `json:command`
	Function    string   `json function,omitempty`
	Zipfile     string   `json zipfile,omitempty`
	Destination string   `json destination,omitempty`
	Sources     []string `json sources,omitempty`
}

func handleJSONRequest(body []byte) {
	var req PostRequest
	err := json.Unmarshal(body, &req)
	if err != nil {
		fmt.Printf("Unmarshal: %v\n", err)
		return
	}
	if req.Command == "s3.concat" {
		if req.Destination == "" || len(req.Sources) == 0 {
			fmt.Printf("need destination and sources\n")
			return
		}
		bucketname := os.Getenv("BUCKET_NAME")
		if bucketname == "" {
			fmt.Printf("no bucket\n")
			return
		}
		b, err := NewBucket(bucketname)
		if err != nil {
			fmt.Printf("NewBucket: %v\n", err)
			return
		}
		if err := b.ConcatObjects(req.Destination, req.Sources); err != nil {
			fmt.Printf("ConcatObjects: %v\n", err)
			return
		}
		fmt.Printf("concat ok\n")
		return
	}
	if req.Command == "lambda.update" {
		if req.Function == "" || req.Zipfile == "" {
			fmt.Printf("need function and zipfile\n")
			return
		}
		bucketname := os.Getenv("BUCKET_NAME")
		if bucketname == "" {
			fmt.Printf("no bucket\n")
			return
		}
		if err := LambdaUpdateFunctionCode(req.Function, bucketname, req.Zipfile); err != nil {
			fmt.Printf("LambdaUpdateFunctionCode: %v\n", err)
			return
		}
		return
	}
}

func handleMultipartRequestSubpart(body []byte) {
	// header and content
	a := bytes.SplitN(body, []byte("\r\n\r\n"), 2)
	if len(a) != 2 {
		// no content
		fmt.Printf("no content\n")
		return
	}
	cdisp := ""
	ctype := ""
	for _, header := range strings.Split(string(a[0]), "\r\n") {
		// use a again
		a := strings.SplitN(header, ": ", 2)
		if len(a) != 2 {
			continue
		}
		key := strings.ToLower(a[0])
		switch key {
		case "content-disposition":
			cdisp = a[1]
		case "content-type":
			ctype = a[1]
		}
	}
	if cdisp == "" {
		fmt.Printf("no Disposition\n")
		return
	}
	fmt.Printf("type: %s disp: %s\n", ctype, cdisp)
	a_disp := strings.Split(cdisp, "; ")
	if a_disp[0] != "form-data" {
		fmt.Printf("unknown Disposition\n")
		return
	}
	name := ""
	filename := ""
	for _, p := range a_disp[1:] {
		// use a again
		a := strings.Split(p, "=")
		if len(a) != 2 {
			continue
		}
		key := strings.ToLower(a[0])
		val := a[1]
		if val[0] == '"' && val[len(val)-1] == '"' {
			val = val[1 : len(val)-1]
		}
		switch key {
		case "name":
			name = val
		case "filename":
			filename = val
		}
	}
	fmt.Printf("name = %s, filename = %s\n", name, filename)
	if name != "file" {
		fmt.Printf("unknown name = %s\n", name)
		return
	}
	// put it in tmp
	bucketname := os.Getenv("BUCKET_NAME")
	if bucketname == "" {
		fmt.Printf("no bucket\n")
		return
	}
	b, err := NewBucket(bucketname)
	if err != nil {
		fmt.Printf("NewBucket: %v\n", err)
		return
	}
	if filename == "" {
		fmt.Printf("no filename\n")
		return
	}
	if err := b.Put("tmp/"+filename, a[1]); err != nil {
		fmt.Printf("S3 Put: %v", err)
		return
	}
}

func handleMultipartRequest(boundary string, body []byte) {
	for n, part := range bytes.Split(body, []byte(boundary)) {
		fmt.Printf("part %d\n", n)
		handleMultipartRequestSubpart(part)
	}
}

// Invoke from Lambda URL
func Handler(req events.LambdaFunctionURLRequest) (string, error) {
	fmt.Println(req)
	switch req.RequestContext.HTTP.Method {
	case "GET":
		return "Lambda works\n", nil
	case "POST": // do nothing
	default:
		return "Unknown request\n", nil
	}
	rawbody := []byte(req.Body)
	if req.IsBase64Encoded {
		rawbody, _ = base64.StdEncoding.DecodeString(req.Body)
	}
	ctype, ok := req.Headers["content-type"]
	if !ok {
		return "No Content-Type\n", nil
	}
	if ctype == "application/json" {
		handleJSONRequest(rawbody)
		return "Done\n", nil
	}
	if strings.Index(ctype, "multipart/form-data; boundary=") == 0 {
		boundary := strings.Split(ctype, "boundary=")[1]
		handleMultipartRequest("--"+boundary, rawbody)
		return "Multipart Done\n", nil
	}
	return "Unknown Content-Type: " + ctype + "\n", nil
}

func main() {
	lambda.Start(Handler)
}
