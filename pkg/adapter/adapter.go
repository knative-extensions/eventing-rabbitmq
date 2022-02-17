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
	"knative.dev/eventing/pkg/kncloudevents"
	"knative.dev/eventing/pkg/metrics/source"
	"knative.dev/pkg/logging"
)

const resourceGroup = "rabbitmqsources.sources.knative.dev"

type ExchangeConfig struct {
	Name       string `envconfig:"RABBITMQ_EXCHANGE_CONFIG_NAME" required:"false"`
	TypeOf     string `envconfig:"RABBITMQ_EXCHANGE_CONFIG_TYPE" required:"true"`
	Durable    bool   `envconfig:"RABBITMQ_EXCHANGE_CONFIG_DURABLE" required:"false"`
	AutoDelete bool   `envconfig:"RABBITMQ_EXCHANGE_CONFIG_AUTO_DELETE" required:"false"`
	Internal   bool   `envconfig:"RABBITMQ_EXCHANGE_CONFIG_INTERNAL" required:"false"`
	NoWait     bool   `envconfig:"RABBITMQ_EXCHANGE_CONFIG_NOWAIT" required:"false"`
}

type ChannelConfig struct {
	PrefetchCount int  `envconfig:"RABBITMQ_CHANNEL_CONFIG_PREFETCH_COUNT" default:"1" required:"false"`
	GlobalQos     bool `envconfig:"RABBITMQ_CHANNEL_CONFIG_QOS_GLOBAL" required:"false"`
}

type QueueConfig struct {
	Name       string `envconfig:"RABBITMQ_QUEUE_CONFIG_NAME" required:"false"`
	RoutingKey string `envconfig:"RABBITMQ_ROUTING_KEY" required:"true"`
	Durable    bool   `envconfig:"RABBITMQ_QUEUE_CONFIG_DURABLE" required:"false"`
	AutoDelete bool   `envconfig:"RABBITMQ_QUEUE_CONFIG_AUTO_DELETE" required:"false"`
	Exclusive  bool   `envconfig:"RABBITMQ_QUEUE_CONFIG_EXCLUSIVE" required:"false"`
	NoWait     bool   `envconfig:"RABBITMQ_QUEUE_CONFIG_NOWAIT" required:"false"`
}

type adapterConfig struct {
	adapter.EnvConfig

	Brokers        string        `envconfig:"RABBITMQ_BROKERS" required:"true"`
	Topic          string        `envconfig:"RABBITMQ_TOPIC" required:"true"`
	User           string        `envconfig:"RABBITMQ_USER" required:"false"`
	Password       string        `envconfig:"RABBITMQ_PASSWORD" required:"false"`
	Vhost          string        `envconfig:"RABBITMQ_VHOST" required:"false"`
	Predeclared    bool          `envconfig:"RABBITMQ_PREDECLARED" required:"false"`
	Retry          int           `envconfig:"RABBITMQ_RETRY" required:"false"`
	BackoffPolicy  string        `envconfig:"RABBITMQ_BACKOFF_POLICY" required:"false"`
	BackoffDelay   time.Duration `envconfig:"RABBITMQ_BACKOFF_DELAY" default:"50ms" required:"false"`
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

func (a *Adapter) CreateConn(user string, password string, logger *zap.Logger) (*amqp.Conn, error) {
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

func (a *Adapter) CreateChannel(conn *amqp.Conn, connTest *amqptest.Conn,
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
		zap.Int("PrefetchCount", a.config.ChannelConfig.PrefetchCount),
		zap.Bool("GlobalQoS", a.config.ChannelConfig.GlobalQos),
	)

	err = ch.Qos(
		a.config.ChannelConfig.PrefetchCount,
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
		zap.String("ExchangeConfigType", a.config.ExchangeConfig.TypeOf),
		zap.String("RoutingKey", a.config.QueueConfig.RoutingKey),
		zap.String("SinkURI", a.config.Sink),
		zap.String("Name", a.config.Name),
		zap.String("Namespace", a.config.Namespace))

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
	logger := a.logger

	if a.config.Predeclared {
		queue, err := (*ch).QueueInspect(a.config.QueueConfig.Name)
		return &queue, err
	}

	exchangeConfig := fillDefaultValuesForExchangeConfig(&a.config.ExchangeConfig, a.config.Topic)

	err := (*ch).ExchangeDeclare(
		exchangeConfig.Name,
		exchangeConfig.TypeOf,
		wabbit.Option{
			"durable":  exchangeConfig.Durable,
			"delete":   exchangeConfig.AutoDelete,
			"internal": exchangeConfig.Internal,
			"noWait":   exchangeConfig.NoWait,
		})
	if err != nil {
		logger.Error(err.Error())
		return nil, err
	}

	queue, err := (*ch).QueueDeclare(
		a.config.QueueConfig.Name,
		wabbit.Option{
			"durable":   a.config.QueueConfig.Durable,
			"delete":    a.config.QueueConfig.AutoDelete,
			"exclusive": a.config.QueueConfig.Exclusive,
			"noWait":    a.config.QueueConfig.NoWait,
		})
	if err != nil {
		logger.Error(err.Error())
		return nil, err
	}

	if a.config.ExchangeConfig.TypeOf != "fanout" {
		routingKeys := strings.Split(a.config.QueueConfig.RoutingKey, ",")

		for _, routingKey := range routingKeys {
			err = (*ch).QueueBind(
				queue.Name(),
				routingKey,
				exchangeConfig.Name,
				wabbit.Option{
					"noWait": a.config.QueueConfig.NoWait,
				})
			if err != nil {
				logger.Error(err.Error())
				return nil, err
			}
		}
	} else {
		err = (*ch).QueueBind(
			queue.Name(),
			"",
			exchangeConfig.Name,
			wabbit.Option{
				"noWait": a.config.QueueConfig.NoWait,
			})
		if err != nil {
			logger.Error(err.Error())
			return nil, err
		}
	}

	return &queue, nil
}

func (a *Adapter) ConsumeMessages(channel *wabbit.Channel,
	queue *wabbit.Queue, logger *zap.Logger) (<-chan wabbit.Delivery, error) {
	msgs, err := (*channel).Consume(
		(*queue).Name(),
		"",
		wabbit.Option{
			"autoAck":   false,
			"exclusive": a.config.QueueConfig.Exclusive,
			"noLocal":   false,
			"noWait":    a.config.QueueConfig.NoWait,
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
	workerCount := a.config.ChannelConfig.PrefetchCount
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
			err = msg.Nack(false, a.config.Retry == 0)
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

	res, err := a.httpMessageSender.SendWithRetries(req, &kncloudevents.RetryConfig{
		RetryMax: a.config.Retry,
		CheckRetry: func(ctx context.Context, resp *nethttp.Response, err error) (bool, error) {
			return kncloudevents.SelectiveRetry(ctx, resp, nil)
		},
		Backoff: func(attemptNum int, resp *nethttp.Response) time.Duration {
			a.logger.Info(fmt.Sprintf("AttempNum #%d send msg %s", attemptNum, msg.MessageId()))
			return a.config.BackoffDelay
		},
	})
	if err != nil {
		a.logger.Error("Error while sending the message", zap.Error(err), zap.Any("%s", req))
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

func fillDefaultValuesForExchangeConfig(config *ExchangeConfig, topic string) *ExchangeConfig {
	if config.TypeOf != "topic" {
		if config.Name == "" {
			config.Name = "logs"
		}
	} else {
		config.Name = topic
	}
	return config
}
