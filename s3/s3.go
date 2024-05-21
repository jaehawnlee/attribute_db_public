package s3

import (
	"context"
	"errors"
	"fmt"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/feature/s3/manager"
	"github.com/aws/aws-sdk-go-v2/service/s3"
)

type S3 struct {
	client *s3.Client
	bucket string
}

type customEndpointResolver struct{}

func (r customEndpointResolver) ResolveEndpoint(service, region string, options ...interface{}) (aws.Endpoint, error) {

	if service == s3.ServiceID {
		// 사용자 지정 엔드포인트 설정
		return aws.Endpoint{
			URL:           "https://kr.object.ncloudstorage.com",
			SigningRegion: region,
		}, nil
	}
	// 기본 리졸버로 대체
	return aws.Endpoint{}, &aws.EndpointNotFoundError{}
}

func NewS3(bucket, region, key, secretKey string, use bool) *S3 {
	var cfg aws.Config
	var err error

	if use {
		cfg, err = config.LoadDefaultConfig(context.TODO(),
			config.WithRegion(region),
			config.WithEndpointResolverWithOptions(customEndpointResolver{}),
			config.WithCredentialsProvider(credentials.NewStaticCredentialsProvider(key, secretKey, "")),
		)
	} else {
		cfg, err = config.LoadDefaultConfig(context.TODO(),
			config.WithRegion(region),
		)
	}

	if err != nil {
		fmt.Println(err)
		return nil
	}

	return &S3{
		client: s3.NewFromConfig(cfg),
		bucket: bucket,
	}
}

func (s *S3) Download(filepath string) ([]byte, error) {
	if s.client == nil {
		return nil, errors.New("S3 Disconnect")
	}

	downloader := manager.NewDownloader(s.client)
	var err error
	for retry := 0; retry < 2; retry++ {
		var data []byte
		data, err = downloadBuffer(context.TODO(), filepath, s.bucket, downloader)
		if err == nil {
			return data, nil
		}
	}

	return nil, err
}

func (s *S3) Write(data []byte, filename string) error {
	if s.client == nil {
		return errors.New("S3 Disconnect")
	}
	uploader := manager.NewUploader(s.client)
	var err error
	for retry := 0; retry < 2; retry++ {
		if err = writeObject(context.TODO(), data, filename, s.bucket, `json`, uploader); err == nil {
			return nil
		}
	}
	return err
}

func (s *S3) WriteImage(data []byte, filename string) error {
	if s.client == nil {
		return errors.New("S3 Disconnect")
	}
	uploader := manager.NewUploader(s.client)
	var err error
	for retry := 0; retry < 2; retry++ {
		if err = writeObject(context.TODO(), data, filename+".jpg", s.bucket, `image/jpeg`, uploader); err == nil {
			return nil
		}
	}
	return err
}

func (s *S3) GetObjectList(path string) ([]string, error) {
	result, err := readInFolder(s.client, s.bucket, path)
	if err == nil {
		list := make([]string, 0)
		for _, bucket := range result {
			list = append(list, *bucket.Key)
		}
		return list, nil
	} else {
		return nil, err
	}
}

func (s *S3) DeleteObject(path string) error {
	return deleteObject(path, s.bucket, s.client)
}
