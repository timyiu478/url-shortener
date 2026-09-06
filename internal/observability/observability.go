package observability

import (
	"context"
	"log/slog"
	"os"
	"time"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	otlptracegrpc "go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.17.0"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

type Config struct {
	ServiceName  string
	OTLPEndpoint string
}

// Init configures structured JSON logging and (optionally) an OTLP gRPC trace exporter.
// Returns a shutdown func to flush/close providers.
func Init(cfg Config) (func(context.Context) error, error) {
	// Structured JSON logging
	jsonHandler := slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{AddSource: false})
	logger := slog.New(jsonHandler)
	slog.SetDefault(logger)

	var tp *sdktrace.TracerProvider
	var cancelFn func()

	if cfg.OTLPEndpoint != "" {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		cancelFn = cancel

		// Dial options (insecure by default for local/dev)
		dialOptions := []grpc.DialOption{grpc.WithTransportCredentials(insecure.NewCredentials())}

		client, err := otlptracegrpc.New(ctx,
			otlptracegrpc.WithEndpoint(cfg.OTLPEndpoint),
			otlptracegrpc.WithDialOption(dialOptions...))
		if err != nil {
			return nil, err
		}

		res, err := resource.New(ctx,
			resource.WithAttributes(
				semconv.ServiceNameKey.String(cfg.ServiceName),
				attribute.String("host.name", os.Getenv("HOSTNAME")),
			),
		)
		if err != nil {
			return nil, err
		}

		tp = sdktrace.NewTracerProvider(
			sdktrace.WithBatcher(client),
			sdktrace.WithResource(res),
		)
		otel.SetTracerProvider(tp)
		logger.Info("OTLP tracer configured", "endpoint", cfg.OTLPEndpoint)
	}

	shutdown := func(ctx context.Context) error {
		if tp != nil {
			ctx2, cancel := context.WithTimeout(ctx, 5*time.Second)
			defer cancel()
			if err := tp.Shutdown(ctx2); err != nil {
				slog.Error("tracer shutdown error", "error", err)
			}
		}
		if cancelFn != nil {
			cancelFn()
		}
		return nil
	}

	return shutdown, nil
}
