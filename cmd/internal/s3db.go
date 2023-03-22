package internal

import (
	"context"
	"fmt"

	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/urfave/cli/v2"
)

func S3Flags() []cli.Flag {
	return []cli.Flag{
		&cli.StringFlag{
			Name:     "s3-bucket",
			Usage:    "S3 Bucket URL",
			EnvVars:  []string{"S3_BUCKET"},
			Required: false,
		},
		&cli.StringFlag{
			Name:     "s3-key",
			Usage:    "S3 Key to use on S3 Bucket",
			EnvVars:  []string{"S3_KEY"},
			Required: false,
		},
	}
}

func CreateS3Client(ctx context.Context) (*s3.Client, error) {
	awscfg, err := config.LoadDefaultConfig(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to load AWS SDK configuration: %w", err)
	}

	return s3.NewFromConfig(awscfg), nil
}
