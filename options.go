package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"os"
	"path"
	"strings"

	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/aws-sdk-go-v2/service/sqs"
	"github.com/aws/aws-sdk-go-v2/service/sqs/types"
)

type Requests interface {
	HasMore() bool
	GetValue() string

	Fetch(fileName string) (string, error)
	Complete(fileName string) error
}

type FileRequest struct {
	inputFiles []string
	outputPath string

	pointer   int
	lastValue string
}

func NewFileRequest(inputFiles []string, outputPath string) (*FileRequest, error) {
	return &FileRequest{
		inputFiles: inputFiles,
		outputPath: strings.TrimSuffix(outputPath, "/"),
	}, nil
}

func (f *FileRequest) HasMore() bool {
	result := false
	if len(f.inputFiles) > f.pointer {
		f.lastValue = f.inputFiles[f.pointer]
		result = true
	}

	f.pointer++
	return result
}

func (f *FileRequest) GetValue() string {
	return f.lastValue
}

func (f *FileRequest) Fetch(filePath string) (string, error) {
	return f.outputPath + string(os.PathSeparator) + path.Base(filePath), nil
}

func (f *FileRequest) Complete(fileName string) error {
	return nil
}

type QueueRequest struct {
	queueName  string
	outputPath string
	tempDir    string

	qClient  *sqs.Client
	s3Client *s3.Client

	nextFile struct {
		fileName  string
		messageId string
	}
}

func NewQueueRequest(queueName, outputDir, tempDir string) (*QueueRequest, error) {
	// this is a shitty place for this
	cfg, err := config.LoadDefaultConfig(context.TODO(), config.WithRegion("us-west-2"))
	if err != nil {
		return nil, errors.New(fmt.Sprintf("Error Loading AWS Config: %s", err))
	}

	// queue client
	qClient := sqs.NewFromConfig(cfg)

	// get the queue url
	qUrlInput := &sqs.GetQueueUrlInput{
		QueueName: &queueName,
	}

	qUrl, err := qClient.GetQueueUrl(context.TODO(), qUrlInput)
	if err != nil {
		return nil, errors.New(fmt.Sprintf("Error Fetching Queue URL: %s", err))
	}

	s3Client := s3.NewFromConfig(cfg)

	return &QueueRequest{
		queueName:  *qUrl.QueueUrl,
		outputPath: outputDir,
		tempDir:    tempDir,

		qClient:  qClient,
		s3Client: s3Client,
	}, nil
}

func (q *QueueRequest) HasMore() bool {
	msgInput := &sqs.ReceiveMessageInput{
		QueueUrl: &q.queueName,
		AttributeNames: []types.QueueAttributeName{
			"SentTimestamp",
		},
		MaxNumberOfMessages: 1,
		MessageAttributeNames: []string{
			"All",
		},
		WaitTimeSeconds: int32(15),
	}

	resp, err := q.qClient.ReceiveMessage(context.TODO(), msgInput)
	if err != nil {
		panic("Error Receiving Message")
	}

	// were fetching messages one at a time
	if len(resp.Messages) == 0 {
		q.nextFile.fileName = ""
		q.nextFile.messageId = ""
		return false
	}

	var Message struct {
		Records []struct {
			S3 struct {
				Bucket struct {
					Name string `json:"name"`
				} `json:"bucket"`
				Object struct {
					Key string `json:"key"`
				} `json:"object"`
			} `json:"s3"`
		} `json:"records"`
	}

	messages := resp.Messages[0]
	if err := json.Unmarshal([]byte(*messages.Body), &Message); err != nil {
		log.Fatalln(err)
	}

	q.nextFile.messageId = *messages.ReceiptHandle
	q.nextFile.fileName = fmt.Sprintf("s3://%s/%s", Message.Records[0].S3.Bucket.Name, Message.Records[0].S3.Object.Key)
	return true
}

func (q *QueueRequest) GetValue() string {
	return q.nextFile.fileName
}

func (q *QueueRequest) Fetch(fileName string) (string, error) {
	// look to see if this is an s3 path
	if isS3Path(fileName) {
		bucketName, objectKey := splitS3Path(fileName)

		getObject := s3.GetObjectInput{
			Bucket: &bucketName,
			Key:    &objectKey,
		}

		response, err := q.s3Client.GetObject(context.TODO(), &getObject)
		if err != nil {
			return "", err
		}

		tempFilePath := q.tempDir + string(os.PathSeparator) + path.Base(objectKey)

		file, _ := os.Create(tempFilePath)
		defer file.Close()

		io.Copy(file, response.Body)

		return tempFilePath, nil
	}

	return "", nil
}

func (q *QueueRequest) Complete(fileName string) error {
	// build the destination file path
	destFilePath := q.outputPath + string(os.PathSeparator) + path.Base(fileName)

	// don't do anything if the source and destination are the same
	if destFilePath == fileName {
		return nil
	}

	// do the s3 thing
	if isS3Path(destFilePath) {
		bucketName, objectKey := splitS3Path(destFilePath)

		filePointer, err := os.Open(fileName)
		if err != nil {
			return err
		}

		defer filePointer.Close()

		putObject := &s3.PutObjectInput{
			Bucket: &bucketName,
			Key:    &objectKey,
			Body:   filePointer,
		}

		if _, err = q.s3Client.PutObject(context.TODO(), putObject); err != nil {
			return err
		}

		// delete the item from the queue
		deleteMessageInput := &sqs.DeleteMessageInput{
			QueueUrl:      &q.queueName,
			ReceiptHandle: &q.nextFile.messageId,
		}

		if _, err = q.qClient.DeleteMessage(context.TODO(), deleteMessageInput); err != nil {
			return err
		}
	} else {
		// otherwise treat it as a file system file
		if _, err := copyFile(fileName, destFilePath); err != nil {
			return err
		}
	}

	// remove the local file
	os.Remove(fileName)

	return nil
}

func FromOptions(opts *Options) (Requests, error) {
	switch opts.Source {
	case "file":
		return NewFileRequest(opts.Input, opts.Output)
	case "queue":
		return NewQueueRequest(opts.Input[0], opts.Output, opts.TempDir)
	default:
		return nil, errors.New(fmt.Sprintf("Invalid Source Provided: %s", opts.Source))
	}
}
