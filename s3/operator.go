package s3

import (
	"bytes"
	"context"
	"fmt"

	"os"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/feature/s3/manager"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/aws-sdk-go-v2/service/s3/types"
)

func readInFolder(service *s3.Client, bucket string, prefix string) ([]types.Object, error) {
	if result, err := service.ListObjectsV2(context.TODO(), &s3.ListObjectsV2Input{
		Bucket:    aws.String(bucket),
		Prefix:    aws.String(prefix + "/"),
		Delimiter: aws.String("/"),
	}); err == nil {
		return result.Contents, nil
	} else {
		return nil, err
	}
}

func downloadObject(ctx context.Context, localPath, item, bucket string, downloader *manager.Downloader) error {
	file, err := os.Create(localPath)
	defer file.Close()
	if err != nil {
		return err
	}

	numBytes, err := downloader.Download(ctx, file,
		&s3.GetObjectInput{
			Bucket: aws.String(bucket),
			Key:    aws.String(item),
		})

	fmt.Println("Downloaded", file.Name(), numBytes, "bytes")
	return err
}

func downloadBuffer(ctx context.Context, item, bucket string, downloader *manager.Downloader) ([]byte, error) {
	buffer := manager.NewWriteAtBuffer([]byte{})
	numBytes, err := downloader.Download(ctx, buffer,
		&s3.GetObjectInput{
			Bucket: aws.String(bucket),
			Key:    aws.String(item),
		})

	if numBytes > 0 && err == nil {
		return buffer.Bytes(), nil
	} else {
		return nil, err
	}
}

func writeObject(ctx context.Context, fileData []byte, filename, bucket, fileType string, uploader *manager.Uploader) error {
	reader := bytes.NewReader(fileData)
	_, err := uploader.Upload(ctx, &s3.PutObjectInput{
		Bucket: aws.String(bucket),
		Key:    aws.String(filename),
		Body:   reader,
	})
	return err
}

func deleteObject(item, bucket string, service *s3.Client) error {
	_, err := service.DeleteObject(context.TODO(), &s3.DeleteObjectInput{
		Bucket: aws.String(bucket),
		Key:    aws.String(item),
	})
	return err
}
