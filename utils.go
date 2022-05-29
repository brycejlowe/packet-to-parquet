package main

import (
	"io"
	"os"
	"strings"
)

func isS3Path(inputPath string) bool {
	if len(inputPath) >= 5 && strings.ToLower(inputPath[0:5]) == "s3://" {
		return true
	}

	return false
}

func splitS3Path(inputPath string) (string, string) {
	if !isS3Path(inputPath) {
		return "", ""
	}

	trimmedString := strings.TrimPrefix(inputPath, "s3://")
	pathParts := strings.SplitN(trimmedString, "/", 2)

	return pathParts[0], pathParts[1]
}

func copyFile(sourcePath, destPath string) (int64, error) {
	sourceFileHandle, err := os.Open(sourcePath)
	if err != nil {
		return -1, err
	}
	defer sourceFileHandle.Close()

	destFileHandle, err := os.Create(destPath)
	if err != nil {
		return -1, err
	}

	defer destFileHandle.Close()

	written, err := io.Copy(destFileHandle, sourceFileHandle)
	if err != nil {
		return -1, nil
	}

	return written, nil
}
