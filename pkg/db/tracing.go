package db

import (
	"context"
	"strings"

	f "github.com/fauna/faunadb-go/v3/faunadb"
	"go.opentelemetry.io/otel/label"
	"go.opentelemetry.io/otel/trace"
)

// Observe sends trace for Fauna requests.
func Observe(ctx context.Context, tracer trace.Tracer) f.ObserverCallback {
	return func(result *f.QueryResult) {
		_, span := tracer.Start(ctx, "fauna_"+strings.SplitN(result.Query.String(), "(", 2)[0], trace.WithTimestamp(result.StartTime))
		defer span.End(trace.WithTimestamp(result.EndTime))

		attributes := []label.KeyValue{
			{Key: label.Key("fauna.request"), Value: label.StringValue(result.Query.String())},
		}

		for key, value := range result.Headers {
			attributeName := strings.ReplaceAll(key, "X-", "fauna.")
			attributeName = strings.ReplaceAll(attributeName, "-", ".")

			if len(value) > 0 {
				attributes = append(attributes, label.KeyValue{
					Key:   label.Key(attributeName),
					Value: label.StringValue(value[0]),
				})
			}
		}
		span.SetAttributes(attributes...)
	}
}

// startSpan starts a new span for tracing and start a Fauna client with a new Observer function.
func (d *FaunaDB) startSpan(ctx context.Context, name string) (f.FaunaClient, trace.Span) {
	ctx, span := d.tracer.Start(ctx, name)

	client := d.client.NewWithObserver(Observe(ctx, d.tracer))

	return *client, span
}
