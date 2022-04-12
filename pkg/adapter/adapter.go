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
	"time"

	"github.com/NeowayLabs/wabbit"
	"github.com/NeowayLabs/wabbit/amqp"
	"github.com/NeowayLabs/wabbit/amqptest"

	cloudevents "github.com/cloudevents/sdk-go/v2"
	"github.com/cloudevents/sdk-go/v2/binding"
	"github.com/cloudevents/sdk-go/v2/protocol/http"

	"go.uber.org/zap"

	"knative.dev/eventing/pkg/adapter/v2"
	v1 "knative.dev/eventing/pkg/apis/duck/v1"
	"knative.dev/eventing/pkg/kncloudevents"
	"knative.dev/eventing/pkg/metrics/source"
	"knative.dev/pkg/logging"
)

const resourceGroup = "rabbitmqsources.sources.knative.dev"

type ExchangeConfig struct {
	Name       string `envconfig:"RABBITMQ_EXCHANGE_CONFIG_NAME" required:"false"`
	Type       string `envconfig:"RABBITMQ_EXCHANGE_CONFIG_TYPE" required:"true"`
	Durable    bool   `envconfig:"RABBITMQ_EXCHANGE_CONFIG_DURABLE" required:"false"`
	AutoDelete bool   `envconfig:"RABBITMQ_EXCHANGE_CONFIG_AUTO_DELETE" required:"false"`
}

type ChannelConfig struct {
	Parallelism int  `envconfig:"RABBITMQ_CHANNEL_CONFIG_PARALLELISM" default:"1" required:"false"`
	GlobalQos   bool `envconfig:"RABBITMQ_CHANNEL_CONFIG_QOS_GLOBAL" required:"false"`
}

type QueueConfig struct {
	Name       string `envconfig:"RABBITMQ_QUEUE_CONFIG_NAME" required:"true"`
	RoutingKey string `envconfig:"RABBITMQ_ROUTING_KEY" required:"true"`
	Durable    bool   `envconfig:"RABBITMQ_QUEUE_CONFIG_DURABLE" required:"false"`
	AutoDelete bool   `envconfig:"RABBITMQ_QUEUE_CONFIG_AUTO_DELETE" required:"false"`
}

type adapterConfig struct {
	adapter.EnvConfig

	Brokers        string        `envconfig:"RABBITMQ_BROKERS" required:"true"`
	User           string        `envconfig:"RABBITMQ_USER" required:"false"`
	Password       string        `envconfig:"RABBITMQ_PASSWORD" required:"false"`
	Vhost          string        `envconfig:"RABBITMQ_VHOST" required:"false"`
	Predeclared    bool          `envconfig:"RABBITMQ_PREDECLARED" required:"false"`
	Retry          int           `envconfig:"HTTP_SENDER_RETRY" required:"false"`
	BackoffPolicy  string        `envconfig:"HTTP_SENDER_BACKOFF_POLICY" required:"false"`
	BackoffDelay   time.Duration `envconfig:"HTTP_SENDER_BACKOFF_DELAY" default:"50ms" required:"false"`
	ChannelConfig  ChannelConfig
	ExchangeConfig ExchangeConfig
	QueueConfig    QueueConfig
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
}

var _ adapter.MessageAdapter = (*Adapter)(nil)
var _ adapter.MessageAdapterConstructor = NewAdapter

func NewAdapter(ctx context.Context, processed adapter.EnvConfigAccessor, httpMessageSender *kncloudevents.HTTPMessageSender, reporter source.StatsReporter) adapter.MessageAdapter {
	logger := logging.FromContext(ctx).Desugar()
	config := processed.(*adapterConfig)
	if config.BackoffPolicy == "" {
		config.BackoffPolicy = "exponential"
	} else if config.BackoffPolicy != "linear" && config.BackoffPolicy != "exponential" {
		logger.Sugar().Fatalf("Invalid BACKOFF_POLICY specified: must be %q or %q", v1.BackoffPolicyExponential, v1.BackoffPolicyLinear)
	}

	return &Adapter{
		config:            config,
		httpMessageSender: httpMessageSender,
		reporter:          reporter,
		logger:            logger,
		context:           ctx,
	}
}

func vhostHandler(brokers string, vhost string) string {
	if len(vhost) > 0 && len(brokers) > 0 && !strings.HasSuffix(brokers, "/") &&
		!strings.HasPrefix(vhost, "/") {
		return fmt.Sprintf("%s/%s", brokers, vhost)
	}

	return fmt.Sprintf("%s%s", brokers, vhost)
}

func (a *Adapter) CreateConn(user string, password string, logger *zap.Logger) (wabbit.Conn, error) {
	if user != "" && password != "" {
		a.config.Brokers = fmt.Sprintf(
			"amqp://%s:%s@%s",
			a.config.User,
			a.config.Password,
			vhostHandler(a.config.Brokers, a.config.Vhost),
		)
	}

	conn, err := amqp.Dial(a.config.Brokers)
	if err != nil {
		logger.Error(err.Error())
	}
	return conn, err
}

func (a *Adapter) CreateChannel(conn wabbit.Conn, connTest *amqptest.Conn,
	logger *zap.Logger) (wabbit.Channel, error) {
	var ch wabbit.Channel
	var err error

	if conn != nil {
		ch, err = conn.Channel()
	} else {
		ch, err = connTest.Channel()
	}
	if err != nil {
		logger.Error(err.Error())
		return nil, err
	}

	logger.Info("Initializing Channel with Config: ",
		zap.Int("Parallelism", a.config.ChannelConfig.Parallelism),
		zap.Bool("GlobalQoS", a.config.ChannelConfig.GlobalQos),
	)

	err = ch.Qos(
		a.config.ChannelConfig.Parallelism,
		0,
		a.config.ChannelConfig.GlobalQos,
	)

	return ch, err
}

func (a *Adapter) Start(ctx context.Context) error {
	return a.start(ctx.Done())
}

func (a *Adapter) start(stopCh <-chan struct{}) error {
	logger := a.logger

	logger.Info("Starting with config: ",
		zap.String("Name", a.config.Name),
		zap.String("Namespace", a.config.Namespace),
		zap.String("QueueName", a.config.QueueConfig.Name),
		zap.String("SinkURI", a.config.Sink))

	conn, err := a.CreateConn(a.config.User, a.config.Password, logger)
	if err == nil {
		defer conn.Close()
	}

	ch, err := a.CreateChannel(conn, nil, logger)
	if err == nil {
		defer ch.Close()
	}

	queue, err := a.StartAmqpClient(&ch)
	if err != nil {
		logger.Error(err.Error())
	}

	return a.PollForMessages(&ch, queue, stopCh)
}

func (a *Adapter) StartAmqpClient(ch *wabbit.Channel) (*wabbit.Queue, error) {
	queue, err := (*ch).QueueInspect(a.config.QueueConfig.Name)
	return &queue, err
}

func (a *Adapter) ConsumeMessages(channel *wabbit.Channel,
	queue *wabbit.Queue, logger *zap.Logger) (<-chan wabbit.Delivery, error) {
	msgs, err := (*channel).Consume(
		(*queue).Name(),
		"",
		wabbit.Option{
			"autoAck":   false,
			"exclusive": false,
			"noLocal":   false,
			"noWait":    false,
		})

	if err != nil {
		logger.Error(err.Error())
	}
	return msgs, err
}

func (a *Adapter) PollForMessages(channel *wabbit.Channel,
	queue *wabbit.Queue, stopCh <-chan struct{}) error {
	logger := a.logger

	msgs, _ := a.ConsumeMessages(channel, queue, logger)

	wg := &sync.WaitGroup{}
	workerCount := a.config.ChannelConfig.Parallelism
	wg.Add(workerCount)
	workerQueue := make(chan wabbit.Delivery, workerCount)
	logger.Info("Starting GoRoutines Workers: ", zap.Int("WorkerCount", workerCount))

	for i := 0; i < workerCount; i++ {
		go a.processMessages(wg, workerQueue)
	}

	for {
		select {
		case <-stopCh:
			close(workerQueue)
			wg.Wait()
			logger.Info("Shutting down...")
			return nil
		case msg, ok := <-msgs:
			if !ok {
				close(workerQueue)
				wg.Wait()
				return nil
			}
			workerQueue <- msg
		}
	}
}

func (a *Adapter) processMessages(wg *sync.WaitGroup, queue <-chan wabbit.Delivery) {
	defer wg.Done()

	for msg := range queue {
		a.logger.Info("Received: ", zap.Any("value", string(msg.Body())))

		if err := a.postMessage(msg); err == nil {
			a.logger.Info("Successfully sent event to sink")
			err = msg.Ack(false)
			if err != nil {
				a.logger.Error("Sending Ack failed with Delivery Tag")
			}
		} else {
			a.logger.Error("Sending event to sink failed: ", zap.Error(err))
			err = msg.Nack(false, false)
			if err != nil {
				a.logger.Error("Sending Nack failed with Delivery Tag")
			}
		}
	}
}

func (a *Adapter) postMessage(msg wabbit.Delivery) error {
	a.logger.Info("url ->" + a.httpMessageSender.Target)
	req, err := a.httpMessageSender.NewCloudEventRequest(a.context)
	if err != nil {
		return err
	}

	var msgBinding binding.Message = NewMessageFromDelivery(msg)

	defer func() {
		err := msgBinding.Finish(nil)
		if err != nil {
			a.logger.Error("Something went wrong while trying to finalizing the message", zap.Error(err))
		}
	}()

	// if the msg is a cloudevent send it as it is to http
	if msgBinding.ReadEncoding() == binding.EncodingUnknown {
		// if the rabbitmq msg is not a cloudevent transform it into one
		event := cloudevents.NewEvent()
		err = convertToCloudEvent(&event, msg, a)
		if err != nil {
			a.logger.Error("Error converting RabbitMQ msg to CloudEvent", zap.Error(err))
			return err
		}

		msgBinding = binding.ToMessage(&event)
	}

	err = http.WriteRequest(a.context, msgBinding, req)
	if err != nil {
		a.logger.Error(fmt.Sprintf("Error writting event to http, encoding: %s", msgBinding.ReadEncoding()), zap.Error(err))
		return err
	}

	backoffDelay := a.config.BackoffDelay.String()
	backoffPolicy := (v1.BackoffPolicyType)(a.config.BackoffPolicy)
	res, err := a.httpMessageSender.SendWithRetries(req, &kncloudevents.RetryConfig{
		RetryMax: a.config.Retry,
		CheckRetry: func(ctx context.Context, resp *nethttp.Response, err error) (bool, error) {
			return kncloudevents.SelectiveRetry(ctx, resp, nil)
		},
		BackoffDelay:  &backoffDelay,
		BackoffPolicy: &backoffPolicy,
	})

	if err != nil {
		a.logger.Error("Error while sending the message", zap.Error(err))
		return err
	}

	if res.StatusCode/100 != 2 {
		a.logger.Error("Unexpected status code", zap.Int("status code", res.StatusCode))
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
