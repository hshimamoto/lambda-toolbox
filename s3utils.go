// MIT License Copyright (C) 2022 Hiroshi Shimamoto
package main

import (
	"bytes"
	"encoding/base64"
	"io"
	"os"
	"path/filepath"
)

func (b *Bucket) Base64DecodeObject(dst, src string) error {
	body, err := b.Get(src)
	if err != nil {
		return err
	}
	decoder := base64.NewDecoder(base64.StdEncoding, bytes.NewBuffer(body))
	plain, err := io.ReadAll(decoder)
	if err != nil {
		return err
	}
	return b.Put(dst, plain)
}

func (b *Bucket) ConcatObjects(dst string, srcs []string) error {
	newobj := new(bytes.Buffer)
	for _, src := range srcs {
		obj, err := b.Get(src)
		if err != nil {
			return err
		}
		if _, err := newobj.Write(obj); err != nil {
			return err
		}
	}
	return b.Put(dst, newobj.Bytes())
}

func (b *Bucket) StoreObject(dst string, srcs []string) error {
	// every file must be in /tmp
	for _, src := range srcs {
		obj, err := os.ReadFile("/tmp/" + src)
		if err != nil {
			return err
		}
		if err := b.Put(filepath.Join(dst, src), obj); err != nil {
			return err
		}
	}
	return nil
}
