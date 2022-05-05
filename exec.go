// MIT License Copyright (C) 2022 Hiroshi Shimamoto
package main

import (
	"archive/zip"
	"bytes"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

func Unzip(zipbytes []byte, dir string) error {
	reader := bytes.NewReader(zipbytes)
	zipreader, err := zip.NewReader(reader, int64(len(zipbytes)))
	if err != nil {
		return err
	}
	// unzip to /tmp/load
	for _, f := range zipreader.File {
		if strings.Index(f.Name, "..") != -1 {
			return fmt.Errorf("bad name %s", f.Name)
		}
		zipf, err := f.Open()
		if err != nil {
			return err
		}
		newpath := filepath.Join(dir, f.Name)
		newf, err := os.OpenFile(newpath, os.O_WRONLY|os.O_CREATE, f.Mode())
		if err != nil {
			zipf.Close()
			return err
		}
		io.Copy(newf, zipf)
		newf.Close()
		zipf.Close()
	}
	return nil
}

func ExecListFiles(dir string) ([]string, error) {
	cmd := exec.Command("ls", "-l", dir)
	output, err := cmd.Output()
	if err != nil {
		return nil, err
	}
	return strings.Split(string(output), "\n"), nil
}
