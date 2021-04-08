package tracer

import (
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/exporters/trace/jaeger"
	"go.opentelemetry.io/otel/propagation"
	export "go.opentelemetry.io/otel/sdk/export/trace"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/semconv"
)

const serviceName = "plugin"

// Setup setups the tracer.
func Setup(exporter export.SpanExporter, probability float64) sdktrace.SpanProcessor {
	bsp := sdktrace.NewBatchSpanProcessor(exporter)

	sampler := sdktrace.ParentBased(sdktrace.TraceIDRatioBased(probability))

	tp := sdktrace.NewTracerProvider(
		sdktrace.WithSampler(sampler),
		sdktrace.WithResource(resource.NewWithAttributes(
			semconv.ServiceNameKey.String(serviceName),
			attribute.String("exporter", "jaeger"),
		)),
		sdktrace.WithSpanProcessor(bsp),
	)

	otel.SetTracerProvider(tp)
	otel.SetTextMapPropagator(propagation.NewCompositeTextMapPropagator(propagation.TraceContext{}))

	return bsp
}

// NewJaegerExporter creates a new Jaeger exporter.
func NewJaegerExporter(endpoint, username, password string) (*jaeger.Exporter, error) {
	return jaeger.NewRawExporter(
		jaeger.WithCollectorEndpoint(endpoint+"/api/traces",
			jaeger.WithUsername(username),
			jaeger.WithPassword(password),
		))
}
