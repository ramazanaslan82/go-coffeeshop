package otel

import (
	"context"
	"net/http"
	"os"
	"time"

	promclient "github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc"
	otelprom "go.opentelemetry.io/otel/exporters/prometheus"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/sdk/resource"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"

	"go.opentelemetry.io/contrib/instrumentation/runtime"
)

// Setup configures global OpenTelemetry tracing (OTLP) and metrics (Prometheus).
// It returns:
//  - shutdown func to flush and close providers
//  - metrics HTTP handler to be mounted at /metrics
func Setup(ctx context.Context, serviceName, serviceVersion string) (func(context.Context) error, http.Handler, error) {
	res, err := resource.New(
		ctx,
		resource.WithFromEnv(),
		resource.WithProcess(),
		resource.WithOS(),
		resource.WithContainer(),
		resource.WithHost(),
		resource.WithAttributes(
			attribute.String("service.name", serviceName),
			attribute.String("service.version", serviceVersion),
		),
	)
	if err != nil {
		return nil, nil, err
	}

	// Propagators (W3C TraceContext + Baggage)
	otel.SetTextMapPropagator(propagation.NewCompositeTextMapPropagator(
		propagation.TraceContext{},
		propagation.Baggage{},
	))

	// Tracing exporter (OTLP/gRPC). Uses OTEL_* env vars if set.
	traceOpts := []otlptracegrpc.Option{}
	if endpoint := os.Getenv("OTEL_EXPORTER_OTLP_ENDPOINT"); endpoint != "" {
		traceOpts = append(traceOpts, otlptracegrpc.WithEndpoint(endpoint))
	}
	if os.Getenv("OTEL_EXPORTER_OTLP_INSECURE") == "true" {
		traceOpts = append(traceOpts, otlptracegrpc.WithInsecure())
	}
	traceExp, err := otlptracegrpc.New(ctx, traceOpts...)
	if err != nil {
		return nil, nil, err
	}
	tp := sdktrace.NewTracerProvider(
		sdktrace.WithBatcher(traceExp),
		sdktrace.WithResource(res),
	)
	otel.SetTracerProvider(tp)

	// Metrics exporter (Prometheus, pull-based)
	// Use a dedicated registry and expose it via promhttp.
	reg := promclient.NewRegistry()
	promExp, err := otelprom.New(
		otelprom.WithRegisterer(reg),
	)
	if err != nil {
		_ = tp.Shutdown(ctx)
		return nil, nil, err
	}
	mp := sdkmetric.NewMeterProvider(
		sdkmetric.WithReader(promExp),
		sdkmetric.WithResource(res),
	)
	otel.SetMeterProvider(mp)

	// Runtime metrics (GC, goroutines, memory, etc.)
	_ = runtime.Start(
		runtime.WithMinimumReadMemStatsInterval(10*time.Second),
		runtime.WithMeterProvider(mp),
	)

	shutdown := func(c context.Context) error {
		// shut down metrics first to ensure readers flush
		_ = mp.Shutdown(c)
		return tp.Shutdown(c)
	}

	// Build an HTTP handler for the custom registry.
	return shutdown, promhttp.HandlerFor(reg, promhttp.HandlerOpts{}), nil
}


