package s3db

import (
	"context"
	"encoding/json"
	"fmt"
	"regexp"
	"sort"
	"strings"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/aws-sdk-go/service/s3/s3iface"
	"github.com/traefik/plugin-service/pkg/db"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/trace"
)

// S3DB is a S3DB client.
type S3DB struct {
	s3Client s3iface.S3API
	s3Bucket string
	s3Key    string
	plugins  map[string]db.Plugin
	tracer   trace.Tracer
}

func NewS3DB(ctx context.Context, s3Client s3iface.S3API, s3Bucket, s3Key string) (*S3DB, error) {
	s3Object, err := s3Client.GetObject(s3.GetObjectInput{
		Bucket: aws.String(s3Bucket),
		Key:    aws.String(s3Key),
	})
	if err != nil {
		return nil, fmt.Errorf("cannot get %s on %s: %w", s3Key, s3Bucket, err)
	}

	defer func() { _ = s3Object.Body.Close() }()
	plugins := make(map[string]db.Plugin)

	decoder := json.NewDecoder(s3Object.Body)
	if err := decoder.Decode(&plugins); err != nil {
		return nil, fmt.Errorf("cannot decode %s on %s: %w", s3Key, s3Bucket, err)
	}

	return &S3DB{
		s3Bucket: s3Bucket,
		s3Client: s3Client,
		s3Key:    s3Key,
		plugins:  plugins,
		tracer:   otel.Tracer("S3Database"),
	}, nil
}

func (s *S3DB) Bootstrap() error {
	return nil
}

func (s *S3DB) Ping(ctx context.Context) error {
	return nil
}

func (s *S3DB) Get(ctx context.Context, id string) (db.Plugin, error) {
	_, span := s.tracer.Start(ctx, "s3db_get")
	defer span.End()

	plugin, present := s.plugins[id]
	if !present {
		return db.Plugin{}, fmt.Errorf("unable to retrieve plugin '%s'", id)
	}

	return plugin, nil
}

func (s *S3DB) Delete(ctx context.Context, id string) error {
	return fmt.Errorf("this is a readonly service")
}
func (s *S3DB) Create(ctx context.Context, plugin db.Plugin) (db.Plugin, error) {
	return db.Plugin{}, fmt.Errorf("this is a readonly service")
}

// TODO: Put ordered plugins list in the struct
func (s *S3DB) List(ctx context.Context, pagination db.Pagination) ([]db.Plugin, string, error) {
	_, span := s.tracer.Start(ctx, "s3db_get")
	defer span.End()

	keys := make([]string, 0, len(s.plugins))

	for key := range s.plugins {
		keys = append(keys, key)
	}
	sort.SliceStable(keys, func(i, j int) bool {
		return s.plugins[keys[i]].Stars < s.plugins[keys[j]].Stars
	})

	plugins := make([]db.Plugin, 0, len(s.plugins))
	for _, k := range keys {
		if plugin := s.plugins[k]; !plugin.Disabled {
			plugins = append(plugins, s.plugins[k])
		}
	}

	return plugins, "", nil
}

func (s *S3DB) GetByName(ctx context.Context, name string, filterDisabled bool) (db.Plugin, error) {
	_, span := s.tracer.Start(ctx, "s3db_get")
	defer span.End()

	for _, plugin := range s.plugins {
		if filterDisabled && !plugin.Disabled {
			continue
		}
		if strings.EqualFold(plugin.Name, name) {
			return plugin, nil
		}
	}

	return db.Plugin{}, fmt.Errorf("plugin '%s' not found", name)
}
func (s *S3DB) SearchByName(ctx context.Context, name string, pagination db.Pagination) ([]db.Plugin, string, error) {
	_, span := s.tracer.Start(ctx, "s3db_get")
	defer span.End()
	var results []db.Plugin

	for _, plugin := range s.plugins {
		if !plugin.Disabled {
			continue
		}
		matched, err := regexp.Match(name, []byte(plugin.DisplayName))
		if err != nil {
			return nil, "", err
		}
		if matched {
			results = append(results, plugin)
		}
	}
	return results, "", nil
}
func (s *S3DB) Update(context.Context, string, db.Plugin) (db.Plugin, error) {
	return db.Plugin{}, fmt.Errorf("this is a readonly service")
}

func (s *S3DB) CreateHash(ctx context.Context, module, version, hash string) (db.PluginHash, error) {
	return db.PluginHash{}, fmt.Errorf("this is a readonly service")
}
func (s *S3DB) GetHashByName(ctx context.Context, module, version string) (db.PluginHash, error) {
	return db.PluginHash{}, fmt.Errorf("not implemented on this store")
}
