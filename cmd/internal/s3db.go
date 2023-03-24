package internal

import (
	"context"
	"fmt"

	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/ettle/strcase"
	"github.com/urfave/cli/v2"
)

const (
	flagS3Bucket = "s3-bucket"
	flagS3Key    = "s3-key"
)

func S3Flags() []cli.Flag {
	return []cli.Flag{
		&cli.StringFlag{
			Name:     flagS3Bucket,
			Usage:    "Bucket to use for loading data",
			EnvVars:  []string{strcase.ToSNAKE(flagS3Bucket)},
			Required: false,
		},
		&cli.StringFlag{
			Name:     flagS3Key,
			Usage:    "Key of file to use on S3 Bucket",
			EnvVars:  []string{strcase.ToSNAKE(flagS3Key)},
			Value:    "plugins.json",
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
