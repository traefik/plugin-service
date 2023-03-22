package s3db

import (
	"context"
	"testing"

	"github.com/aws/aws-sdk-go/service/s3"
	"github.com/aws/aws-sdk-go/service/s3/s3iface"
)

type mockS3Client struct {
	s3iface.S3API
}

func (m *mockS3Client) GetObject(input *s3.GetObjectInput) (*s3.GetObjectOutput, error) {
	// mock response/functionality
	return nil, nil
}

func TestS3DB_Create(t *testing.T) {
	client := &mockS3Client{}
	s3db, err := NewS3DB(context.Background(), client, "bucket", "key")
}
