package tracer

import (
	"net/http"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/api/global"
	"go.opentelemetry.io/otel/exporters/trace/jaeger"
	"go.opentelemetry.io/otel/label"
	"go.opentelemetry.io/otel/propagators"
	export "go.opentelemetry.io/otel/sdk/export/trace"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
)

const serviceName = "plugin"

// Setup setups the tracer.
func Setup(exporter export.SpanExporter, probability float64) *sdktrace.BatchSpanProcessor {
	bsp := sdktrace.NewBatchSpanProcessor(exporter)

	tp := sdktrace.NewTracerProvider(
		sdktrace.WithConfig(sdktrace.Config{DefaultSampler: sdktrace.TraceIDRatioBased(probability)}),
		sdktrace.WithSpanProcessor(bsp),
	)

	global.SetTracerProvider(tp)
	global.SetTextMapPropagator(otel.NewCompositeTextMapPropagator(propagators.TraceContext{}))

	return bsp
}

// NewJaegerExporter creates a new Jaeger exporter.
func NewJaegerExporter(req *http.Request, endpoint, username, password string) (*jaeger.Exporter, error) {
	return jaeger.NewRawExporter(
		jaeger.WithCollectorEndpoint(endpoint+"/api/traces",
			jaeger.WithUsername(username),
			jaeger.WithPassword(password),
		),
		jaeger.WithProcess(jaeger.Process{
			ServiceName: serviceName,
			Tags: []label.KeyValue{
				label.String("exporter", "jaeger"),
				label.String("region", req.Header.Get("x-vercel-id")),
				label.String("deployment", req.Header.Get("x-vercel-deployment-url")),
			},
		}))
}
