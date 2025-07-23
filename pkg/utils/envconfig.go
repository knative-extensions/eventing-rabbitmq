package utils

import (
	"context"
	"encoding/json"
	"fmt"

	"go.opentelemetry.io/contrib/instrumentation/runtime"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/trace"
	"go.uber.org/zap"
	"knative.dev/eventing/pkg/observability"
	eventingotel "knative.dev/eventing/pkg/observability/otel"
	"knative.dev/eventing/pkg/observability/resource"
	"knative.dev/pkg/logging"
	"knative.dev/pkg/observability/metrics"
	k8sruntime "knative.dev/pkg/observability/runtime/k8s"
	"knative.dev/pkg/observability/tracing"
)

type EnvConfig struct {
	// LoggingConfigJson is a json string of logging.Config.
	// This is used to configure the logging config, the config is stored in
	// a config map inside the controllers namespace and copied here.
	LoggingConfigJson string `envconfig:"K_LOGGING_CONFIG" default:"{}"`

	// ObservabilityConfigJson is a json string of observability.Config.
	// This is used to configure the tracing config, the config is stored in
	// a config map inside the controllers namespace and copied here.
	// Default is no-op.
	ObservabilityConfigJson string `envconfig:"K_OBSERVABILITY_CONFIG" default:"{}"`

	component string
	logger    *zap.SugaredLogger

	meterProvider *metrics.MeterProvider
	traceProvider *tracing.TracerProvider
}

func (e *EnvConfig) SetComponent(component string) {
	e.component = component
}

func (e *EnvConfig) GetLogger() *zap.SugaredLogger {
	if e.logger == nil {
		loggingConfig, err := logging.JSONToConfig(e.LoggingConfigJson)
		if err != nil {
			// Use default logging config.
			if loggingConfig, err = logging.NewConfigFromMap(map[string]string{}); err != nil {
				// If this fails, there is no recovering.
				panic(err)
			}
		}

		logger, _ := logging.NewLoggerFromConfig(loggingConfig, e.component)
		e.logger = logger
	}
	return e.logger
}

func (e *EnvConfig) SetupObservability(ctx context.Context) error {
	cfg := &observability.Config{}
	err := json.Unmarshal([]byte(e.ObservabilityConfigJson), cfg)
	if err != nil {
		return fmt.Errorf("failed to parse observability config from env: %w", err)
	}

	cfg = observability.MergeWithDefaults(cfg)

	logger := e.GetLogger()

	pprof := k8sruntime.NewProfilingServer(logger.Named("pprof"))

	pprof.UpdateFromConfig(cfg.Runtime)

	otelResource, err := resource.Default(e.component)
	if err != nil {
		logger.Warnw("encountered an error creating otel resource, some attributes may not be set", zap.Error(err))
	}

	meterProvider, err := metrics.NewMeterProvider(
		ctx,
		cfg.Metrics,
		metric.WithResource(otelResource),
	)
	if err != nil {
		logger.Warnw("encountered error setting up otel meter provider, falling back to noop default", zap.Error(err))
		meterProvider = eventingotel.DefaultMeterProvider(ctx, otelResource)
	}

	otel.SetMeterProvider(meterProvider)

	err = runtime.Start(
		runtime.WithMinimumReadMemStatsInterval(cfg.Runtime.ExportInterval),
	)
	if err != nil {
		logger.Warnw("failed to start runtime metrics, will not export them", zap.Error(err))
	}

	tracerProvider, err := tracing.NewTracerProvider(
		ctx,
		cfg.Tracing,
		trace.WithResource(otelResource),
	)
	if err != nil {
		logger.Warnw("encountered error setting up otel tracer provider, falling back to noop default", zap.Error(err))
		tracerProvider = eventingotel.DefaultTraceProvider(ctx, otelResource)
	}

	otel.SetTextMapPropagator(tracing.DefaultTextMapPropagator())
	otel.SetTracerProvider(tracerProvider)

	e.meterProvider = meterProvider
	e.traceProvider = tracerProvider

	return nil
}

func (e *EnvConfig) ShutdownObservability(ctx context.Context) {
	logger := e.GetLogger()
	if err := e.meterProvider.Shutdown(ctx); err != nil {
		logger.Errorw("error flushing metrics", zap.Error(err))
	}

	if err := e.traceProvider.Shutdown(ctx); err != nil {
		logger.Errorw("error flushing traces", zap.Error(err))
	}
}
