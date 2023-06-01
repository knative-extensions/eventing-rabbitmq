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
	"sync"
	"time"

	"go.uber.org/zap"

	amqp "github.com/rabbitmq/amqp091-go"

	"knative.dev/eventing-rabbitmq/pkg/rabbit"
	"knative.dev/eventing-rabbitmq/pkg/utils"
	"knative.dev/eventing/pkg/adapter/v2"
	v1 "knative.dev/eventing/pkg/apis/duck/v1"
	"knative.dev/eventing/pkg/kncloudevents"
	"knative.dev/eventing/pkg/metrics/source"
	duckv1 "knative.dev/pkg/apis/duck/v1"
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
	BackoffDelay  string `envconfig:"HTTP_SENDER_BACKOFF_DELAY" default:"PT0.2S" required:"false"`
	Parallelism   int    `envconfig:"RABBITMQ_CHANNEL_PARALLELISM" default:"1" required:"false"`
	ExchangeName  string `envconfig:"RABBITMQ_EXCHANGE_NAME" required:"false"`
	QueueName     string `envconfig:"RABBITMQ_QUEUE_NAME" required:"true"`
}

func NewEnvConfig() adapter.EnvConfigAccessor {
	return &adapterConfig{}
}

type Adapter struct {
	config            *adapterConfig
	sink              duckv1.Addressable
	httpMessageSender *kncloudevents.HTTPMessageSender
	reporter          source.StatsReporter
	logger            *zap.Logger
	context           context.Context
	rmqHelper         rabbit.RabbitMQConnectionsHandlerInterface
}

var _ adapter.MessageAdapter = (*Adapter)(nil)
var _ adapter.MessageAdapterConstructor = NewAdapter
var (
	retryConfig  *kncloudevents.RetryConfig
	retriesInt32 int32 = 0
)

func NewAdapter(ctx context.Context, processed adapter.EnvConfigAccessor, sink duckv1.Addressable, reporter source.StatsReporter) adapter.MessageAdapter {
	logger := logging.FromContext(ctx).Desugar()
	config := processed.(*adapterConfig)
	httpMessageSender, err := kncloudevents.NewHTTPMessageSenderWithTarget(sink.URL.String())
	if err != nil {
		logger.Sugar().Fatalf("Couldn't start message sender, %w", err)
	}
	return &Adapter{
		config:            config,
		sink:              sink,
		httpMessageSender: httpMessageSender,
		reporter:          reporter,
		logger:            logger,
		context:           ctx,
	}
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
		a.rmqHelper = rabbit.NewRabbitMQConnectionHandler(1000, logger)
		a.rmqHelper.Setup(a.context, rabbit.VHostHandler(a.config.RabbitURL, a.config.Vhost), rabbit.ChannelQoS, rabbit.DialWrapper)
	}
	return a.PollForMessages(stopCh)
}

func (a *Adapter) ConsumeMessages(queue *amqp.Queue, channel rabbit.RabbitMQChannelInterface, logger *zap.SugaredLogger) (<-chan amqp.Delivery, error) {
	if channel != nil {
		msgs, err := channel.Consume(
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
	} else {
		return nil, amqp.ErrClosed
	}
}

func (a *Adapter) PollForMessages(stopCh <-chan struct{}) error {
	logger := a.logger.Sugar()
	var err error
	var queue amqp.Queue
	var msgs <-chan (amqp.Delivery)
	auxRetryConfig := kncloudevents.NoRetries()
	if a.config.BackoffDelay != "" {
		retriesInt32 = int32(a.config.Retry)
		backoffPolicy := utils.SetBackoffPolicy(a.context, a.config.BackoffPolicy)
		if backoffPolicy == "" {
			a.logger.Sugar().Fatalf("Invalid BACKOFF_POLICY specified: must be %q or %q", v1.BackoffPolicyExponential, v1.BackoffPolicyLinear)
		}
		auxRetryConfig, err = kncloudevents.RetryConfigFromDeliverySpec(v1.DeliverySpec{
			BackoffPolicy: &backoffPolicy,
			BackoffDelay:  &a.config.BackoffDelay,
			Retry:         &retriesInt32,
		})
		if err != nil {
			a.logger.Error("error retrieving retryConfig from deliverySpec", zap.Error(err))
		}
	}

	retryConfig = &auxRetryConfig
	wg := &sync.WaitGroup{}
	workerCount := a.config.Parallelism
	wg.Add(workerCount)
	workerQueue := make(chan amqp.Delivery, workerCount)
	logger.Info("Starting GoRoutines Workers: ", zap.Int("WorkerCount", workerCount))

	for i := 0; i < workerCount; i++ {
		go a.processMessages(wg, workerQueue)
	}

	for {
		if channel := a.rmqHelper.GetChannel(); channel != nil {
			if queue, err = channel.QueueInspect(a.config.QueueName); err == nil {
				connNotifyChannel, chNotifyChannel := a.rmqHelper.GetConnection().NotifyClose(make(chan *amqp.Error)), channel.NotifyClose(make(chan *amqp.Error))
				if msgs, err = a.ConsumeMessages(&queue, channel, logger); err == nil {
				loop:
					for {
						select {
						case <-stopCh:
							close(workerQueue)
							wg.Wait()
							logger.Info("Shutting down...")
							return nil
						case <-connNotifyChannel:
							break loop
						case <-chNotifyChannel:
							break loop
						case msg, ok := <-msgs:
							if !ok {
								break loop
							}
							workerQueue <- msg
						}
					}
				}
			}
		}
		if err == nil {
			err = amqp.ErrClosed
		}
		logger.Error(err)
		time.Sleep(time.Second)
	}
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

	res, err := a.httpMessageSender.SendWithRetries(req, retryConfig)
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
