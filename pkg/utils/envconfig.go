package utils

import (
	"context"
	"net/http"
	"strings"

	"go.uber.org/zap"
	"knative.dev/pkg/logging"
	"knative.dev/pkg/metrics"
	"knative.dev/pkg/profiling"
	"knative.dev/pkg/tracing"
	"knative.dev/pkg/tracing/config"
)

type EnvConfig struct {
	// MetricsConfigJson is a json string of metrics.ExporterOptions.
	// This is used to configure the metrics exporter options,
	// the config is stored in a config map inside the controllers
	// namespace and copied here.
	MetricsConfigJson string `envconfig:"K_METRICS_CONFIG" default:"{}"`

	// LoggingConfigJson is a json string of logging.Config.
	// This is used to configure the logging config, the config is stored in
	// a config map inside the controllers namespace and copied here.
	LoggingConfigJson string `envconfig:"K_LOGGING_CONFIG" default:"{}"`

	// TracingConfigJson is a json string of tracing.Config.
	// This is used to configure the tracing config, the config is stored in
	// a config map inside the controllers namespace and copied here.
	// Default is no-op.
	TracingConfigJson string `envconfig:"K_TRACING_CONFIG"`

	component string
	logger    *zap.SugaredLogger
}

func (e *EnvConfig) SetComponent(component string) {
	e.component = component
}

func (e *EnvConfig) SetupTracing() error {
	logger := e.GetLogger()

	config, err := config.JSONToTracingConfig(e.TracingConfigJson)
	if err != nil {
		logger.Warn("Tracing configuration is invalid, using the no-op default", zap.Error(err))
	}
	return tracing.SetupStaticPublishing(logger, "", config)
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

func (e *EnvConfig) SetupMetrics(ctx context.Context) error {
	logger := e.GetLogger()
	// Convert json metrics.ExporterOptions to metrics.ExporterOptions.
	metricsConfig, err := metrics.JSONToOptions(e.MetricsConfigJson)
	if err != nil {
		return err
	}

	if metricsConfig != nil {
		metricsConfig.Component = strings.ReplaceAll(e.component, "-", "_")
		if err := metrics.UpdateExporter(ctx, *metricsConfig, logger); err != nil {
			return err
		}
		// Check if metrics config contains profiling flag
		if metricsConfig.ConfigMap != nil {
			if enabled, err := profiling.ReadProfilingFlag(metricsConfig.ConfigMap); err == nil && enabled {
				// Start a goroutine to server profiling metrics
				logger.Info("Profiling enabled")
				go func() {
					server := profiling.NewServer(profiling.NewHandler(logger, true))
					// Don't forward ErrServerClosed as that indicates we're already shutting down.
					if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
						logger.Errorw("Profiling server failed", zap.Error(err))
					}
				}()
			}
		}
	}
	return nil
}
