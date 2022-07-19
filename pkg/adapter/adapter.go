/*
Copyright 2020 The Knative Authors

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package rabbitmq

import (
	"context"
	"fmt"
	nethttp "net/http"
	"strings"
	"sync"

	"go.uber.org/zap"

	amqp "github.com/rabbitmq/amqp091-go"

	"knative.dev/eventing-rabbitmq/pkg/rabbit"
	"knative.dev/eventing-rabbitmq/pkg/utils"
	"knative.dev/eventing/pkg/adapter/v2"
	v1 "knative.dev/eventing/pkg/apis/duck/v1"
	"knative.dev/eventing/pkg/kncloudevents"
	"knative.dev/eventing/pkg/metrics/source"
	"knative.dev/pkg/logging"
)

const resourceGroup = "rabbitmqsources.sources.knative.dev"

type adapterConfig struct {
	adapter.EnvConfig

	RabbitURL     string `envconfig:"RABBIT_URL" required:"true"`
	Vhost         string `envconfig:"RABBITMQ_VHOST" required:"false"`
	Predeclared   bool   `envconfig:"RABBITMQ_PREDECLARED" required:"false"`
	Retry         int    `envconfig:"HTTP_SENDER_RETRY" required:"false"`
	BackoffPolicy string `envconfig:"HTTP_SENDER_BACKOFF_POLICY" required:"false"`
	BackoffDelay  string `envconfig:"HTTP_SENDER_BACKOFF_DELAY" default:"50ms" required:"false"`
	Parallelism   int    `envconfig:"RABBITMQ_CHANNEL_PARALLELISM" default:"1" required:"false"`
	ExchangeName  string `envconfig:"RABBITMQ_EXCHANGE_NAME" required:"false"`
	QueueName     string `envconfig:"RABBITMQ_QUEUE_NAME" required:"true"`
}

func NewEnvConfig() adapter.EnvConfigAccessor {
	return &adapterConfig{}
}

type Adapter struct {
	config            *adapterConfig
	httpMessageSender *kncloudevents.HTTPMessageSender
	reporter          source.StatsReporter
	logger            *zap.Logger
	context           context.Context
	rmqHelper         rabbit.RabbitMQHelperInterface
	connection        rabbit.RabbitMQConnectionInterface
	channel           rabbit.RabbitMQChannelInterface
}

var _ adapter.MessageAdapter = (*Adapter)(nil)
var _ adapter.MessageAdapterConstructor = NewAdapter
var (
	retryConfig  kncloudevents.RetryConfig = kncloudevents.NoRetries()
	retriesInt32 int32                     = 0
)

func NewAdapter(ctx context.Context, processed adapter.EnvConfigAccessor, httpMessageSender *kncloudevents.HTTPMessageSender, reporter source.StatsReporter) adapter.MessageAdapter {
	logger := logging.FromContext(ctx).Desugar()
	config := processed.(*adapterConfig)
	return &Adapter{
		config:            config,
		httpMessageSender: httpMessageSender,
		reporter:          reporter,
		logger:            logger,
		context:           ctx,
	}
}

func vhostHandler(broker string, vhost string) string {
	if len(vhost) > 0 && len(broker) > 0 && !strings.HasSuffix(broker, "/") &&
		!strings.HasPrefix(vhost, "/") {
		return fmt.Sprintf("%s/%s", broker, vhost)
	}

	return fmt.Sprintf("%s%s", broker, vhost)
}

func (a *Adapter) Start(ctx context.Context) error {
	return a.start(ctx.Done())
}

func (a *Adapter) start(stopCh <-chan struct{}) error {
	logger := a.logger.Sugar()
	logger.Info("Starting with config: ",
		zap.String("Name", a.config.Name),
		zap.String("Namespace", a.config.Namespace),
		zap.String("QueueName", a.config.QueueName),
		zap.String("SinkURI", a.config.Sink))

	if a.rmqHelper == nil {
		a.rmqHelper = rabbit.NewRabbitMQHelper(1, make(chan bool), rabbit.DialWrapper)
	}
	return a.PollForMessages(stopCh)
}

func (a *Adapter) CreateRabbitMQConnections(
	rmqHelper rabbit.RabbitMQHelperInterface,
	logger *zap.SugaredLogger) (connection rabbit.RabbitMQConnectionInterface, channel rabbit.RabbitMQChannelInterface, err error) {
	connection, channel, err = rmqHelper.SetupRabbitMQ(vhostHandler(a.config.RabbitURL, a.config.Vhost), rabbit.ChannelQoS, logger)
	if err != nil {
		rmqHelper.CloseRabbitMQConnections(connection, logger)
		logger.Warn("Retrying RabbitMQ connections setup")
		go rmqHelper.SignalRetry(true)
		return nil, nil, err
	}
	return connection, channel, nil
}

func (a *Adapter) ConsumeMessages(queue *amqp.Queue, logger *zap.SugaredLogger) (<-chan amqp.Delivery, error) {
	msgs, err := a.channel.Consume(
		queue.Name,
		"",
		false,
		false,
		false,
		false,
		amqp.Table{})

	if err != nil {
		logger.Error(err.Error())
	}
	return msgs, err
}

func (a *Adapter) PollForMessages(stopCh <-chan struct{}) error {
	logger := a.logger.Sugar()
	var err error
	var queue amqp.Queue
	var msgs <-chan (amqp.Delivery)
	if a.config.BackoffDelay != "" {
		retriesInt32 = int32(a.config.Retry)
		backoffPolicy := utils.SetBackoffPolicy(a.context, a.config.BackoffPolicy)
		if backoffPolicy == "" {
			a.logger.Sugar().Fatalf("Invalid BACKOFF_POLICY specified: must be %q or %q", v1.BackoffPolicyExponential, v1.BackoffPolicyLinear)
		}
		retryConfig, err = kncloudevents.RetryConfigFromDeliverySpec(v1.DeliverySpec{
			BackoffPolicy: &backoffPolicy,
			BackoffDelay:  &a.config.BackoffDelay,
			Retry:         &retriesInt32,
		})
		if err != nil {
			a.logger.Error("error retrieving retryConfig from deliverySpec", zap.Error(err))
		}
	}

	wg := &sync.WaitGroup{}
	workerCount := a.config.Parallelism
	wg.Add(workerCount)
	workerQueue := make(chan amqp.Delivery, workerCount)
	logger.Info("Starting GoRoutines Workers: ", zap.Int("WorkerCount", workerCount))

	for i := 0; i < workerCount; i++ {
		go a.processMessages(wg, workerQueue)
	}

	defer a.rmqHelper.CleanupRabbitMQ(a.connection, logger)
	for {
		a.connection, a.channel, err = a.CreateRabbitMQConnections(a.rmqHelper, logger)
		if err != nil {
			logger.Errorf("error creating RabbitMQ connections: %s, waiting for a retry", err)
		}
		queue, err = a.channel.QueueInspect(a.config.QueueName)
		if err != nil {
			logger.Error(err.Error())
		}
		if err != nil {
			continue
		}

		msgs, _ = a.ConsumeMessages(&queue, logger)
		go PollCycle(stopCh, workerQueue, wg, msgs, a.rmqHelper, logger)
		if retry := a.rmqHelper.WaitForRetrySignal(); !retry {
			logger.Warn("stopped listenning for RabbitMQ resources retries")
			return nil
		}
		logger.Warn("recreating RabbitMQ resources")
	}
}

func PollCycle(
	stopCh <-chan struct{},
	workerQueue chan amqp.Delivery,
	wg *sync.WaitGroup,
	msgs <-chan amqp.Delivery,
	rmqHelper rabbit.RabbitMQHelperInterface,
	logger *zap.SugaredLogger) {
	for {
		select {
		case <-stopCh:
			shutDown(workerQueue, wg, rmqHelper, logger)
			return
		case msg, ok := <-msgs:
			if !ok {
				shutDown(workerQueue, wg, rmqHelper, logger)
				return
			}
			workerQueue <- msg
		}
	}
}

func shutDown(workerQueue chan amqp.Delivery, wg *sync.WaitGroup, rmqHelper rabbit.RabbitMQHelperInterface, logger *zap.SugaredLogger) {
	close(workerQueue)
	wg.Wait()
	logger.Info("Shutting down...")
	rmqHelper.SignalRetry(false)
}

func (a *Adapter) processMessages(wg *sync.WaitGroup, queue <-chan amqp.Delivery) {
	defer wg.Done()
	for msg := range queue {
		a.logger.Info("Received: ", zap.String("MessageId", msg.MessageId))
		if err := a.postMessage(&msg); err == nil {
			a.logger.Info("Successfully sent event to sink")
			err = msg.Ack(false)
			if err != nil {
				a.logger.Error("sending Ack failed with Delivery Tag")
			}
		} else {
			a.logger.Error("sending event to sink failed: ", zap.Error(err))
			err = msg.Nack(false, false)
			if err != nil {
				a.logger.Error("sending Nack failed with Delivery Tag")
			}
		}
	}
}

func (a *Adapter) postMessage(msg *amqp.Delivery) error {
	a.logger.Info("target: " + a.httpMessageSender.Target)
	req, err := a.httpMessageSender.NewCloudEventRequest(a.context)
	if err != nil {
		return err
	}

	err = rabbit.ConvertMessageToHTTPRequest(
		a.context,
		a.config.Name,
		a.config.Namespace,
		a.config.QueueName,
		msg,
		req,
		a.logger)
	if err != nil {
		a.logger.Error("error writing event to http", zap.Error(err))
		return err
	}

	res, err := a.httpMessageSender.SendWithRetries(req, &retryConfig)
	if err != nil {
		a.logger.Error("error while sending the message", zap.Error(err))
		return err
	}

	if res.StatusCode/100 != 2 {
		a.logger.Error("unexpected status code", zap.Int("status code", res.StatusCode))
		return fmt.Errorf("%d %s", res.StatusCode, nethttp.StatusText(res.StatusCode))
	}

	reportArgs := &source.ReportArgs{
		Namespace:     a.config.Namespace,
		Name:          a.config.Name,
		ResourceGroup: resourceGroup,
	}

	_ = a.reporter.ReportEventCount(reportArgs, res.StatusCode)
	return nil
}
