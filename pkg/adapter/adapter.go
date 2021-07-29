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
	"encoding/json"
	"fmt"
	nethttp "net/http"
	"strings"

	"github.com/NeowayLabs/wabbit"
	"github.com/NeowayLabs/wabbit/amqp"
	"github.com/NeowayLabs/wabbit/amqptest"
	cloudevents "github.com/cloudevents/sdk-go/v2"
	"github.com/cloudevents/sdk-go/v2/binding"
	cloudevent "github.com/cloudevents/sdk-go/v2/event"
	"github.com/cloudevents/sdk-go/v2/protocol/http"
	"go.uber.org/zap"
	"k8s.io/apimachinery/pkg/util/uuid"
	sourcesv1alpha1 "knative.dev/eventing-rabbitmq/pkg/apis/sources/v1alpha1"
	"knative.dev/eventing/pkg/adapter/v2"
	"knative.dev/eventing/pkg/kncloudevents"
	"knative.dev/eventing/pkg/metrics/source"
	"knative.dev/pkg/logging"
)

const (
	resourceGroup = "rabbitmqsources.sources.knative.dev"
)

type ExchangeConfig struct {
	Name        string `envconfig:"RABBITMQ_EXCHANGE_CONFIG_NAME" required:"false"`
	TypeOf      string `envconfig:"RABBITMQ_EXCHANGE_CONFIG_TYPE" required:"true"`
	Durable     bool   `envconfig:"RABBITMQ_EXCHANGE_CONFIG_DURABLE" required:"false"`
	AutoDeleted bool   `envconfig:"RABBITMQ_EXCHANGE_CONFIG_AUTO_DELETED" required:"false"`
	Internal    bool   `envconfig:"RABBITMQ_EXCHANGE_CONFIG_INTERNAL" required:"false"`
	NoWait      bool   `envconfig:"RABBITMQ_EXCHANGE_CONFIG_NOWAIT" required:"false"`
}

type ChannelConfig struct {
	PrefetchCount int
	GlobalQos     bool
}

type QueueConfig struct {
	Name             string `envconfig:"RABBITMQ_QUEUE_CONFIG_NAME" required:"false"`
	RoutingKey       string `envconfig:"RABBITMQ_ROUTING_KEY" required:"true"`
	Durable          bool   `envconfig:"RABBITMQ_QUEUE_CONFIG_DURABLE" required:"false"`
	DeleteWhenUnused bool   `envconfig:"RABBITMQ_QUEUE_CONFIG_AUTO_DELETED" required:"false"`
	Exclusive        bool   `envconfig:"RABBITMQ_QUEUE_CONFIG_EXCLUSIVE" required:"false"`
	NoWait           bool   `envconfig:"RABBITMQ_QUEUE_CONFIG_NOWAIT" required:"false"`
}

type adapterConfig struct {
	adapter.EnvConfig

	Brokers        string `envconfig:"RABBITMQ_BROKERS" required:"true"`
	Topic          string `envconfig:"RABBITMQ_TOPIC" required:"true"`
	User           string `envconfig:"RABBITMQ_USER" required:"false"`
	Password       string `envconfig:"RABBITMQ_PASSWORD" required:"false"`
	Vhost          string `envconfig:"RABBITMQ_VHOST" required:"false"`
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

	err = ch.Qos(a.config.ChannelConfig.PrefetchCount,
		0,
		a.config.ChannelConfig.GlobalQos)

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
	exchangeConfig := fillDefaultValuesForExchangeConfig(&a.config.ExchangeConfig, a.config.Topic)

	err := (*ch).ExchangeDeclare(
		exchangeConfig.Name,
		exchangeConfig.TypeOf,
		wabbit.Option{
			"durable":  exchangeConfig.Durable,
			"delete":   exchangeConfig.AutoDeleted,
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
			"delete":    a.config.QueueConfig.DeleteWhenUnused,
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

	for {
		select {
		case msg, ok := <-msgs:
			if ok {
				logger.Info("Received: ", zap.Any("value", string(msg.Body())))

				if err := a.postMessage(msg); err == nil {
					logger.Info("Successfully sent event to sink")
					err = msg.Ack(false)
					if err != nil {
						logger.Error("Sending Ack failed with Delivery Tag")
					}
				} else {
					logger.Error("Sending event to sink failed: ", zap.Error(err))
					err = msg.Nack(false, true)
					if err != nil {
						logger.Error("Sending Nack failed with Delivery Tag")
					}
				}
			} else {
				return nil
			}
		case <-stopCh:
			logger.Info("Shutting down...")
			return nil
		}
	}
}

func msgContentType(msg wabbit.Delivery) (string, error) {
	var contentType string

	for key, val := range msg.Headers() {
		if strings.ToLower(key) == "content-type" {
			contentType = strings.ToLower(fmt.Sprint(val))
		}
	}

	if contentType == "" {
		return "", fmt.Errorf("no content type present on Rabbitmq msg")
	}

	switch contentType {
	case cloudevent.ApplicationCloudEventsJSON, cloudevent.ApplicationCloudEventsBatchJSON:
		return contentType, nil
	default:
		return cloudevent.ApplicationJSON, nil
	}
}

func setEventContent(a *Adapter, msg wabbit.Delivery, contentType string) (cloudevent.Event, error) {
	event := cloudevents.NewEvent()
	if contentType == cloudevent.ApplicationCloudEventsJSON {
		err := json.Unmarshal(msg.Body(), &event)
		if err != nil {
			return event, err
		}
	} else {
		if msg.MessageId() != "" {
			event.SetID(msg.MessageId())
		} else {
			event.SetID(string(uuid.NewUUID()))
		}

		event.SetTime(msg.Timestamp())
		event.SetType(sourcesv1alpha1.RabbitmqEventType)
		event.SetSource(sourcesv1alpha1.RabbitmqEventSource(
			a.config.Namespace,
			a.config.Name,
			a.config.Topic,
		))
		event.SetSubject(msg.MessageId())
		event.SetExtension("key", msg.MessageId())

		err := event.SetData(contentType, msg.Body())
		if err != nil {
			return event, err
		}
	}

	return event, nil
}

func setEventBatchContent(a *Adapter, msg wabbit.Delivery) ([]cloudevent.Event, error) {
	// Would be good to have a batch size parameter
	// Process all the events, so it does not send any events
	var payload []cloudevent.Event
	err := json.Unmarshal(msg.Body(), &payload)
	if err != nil {
		return payload, err
	}

	return payload, nil
}

func (a *Adapter) postMessage(msg wabbit.Delivery) error {
	a.logger.Info("url ->" + a.httpMessageSender.Target)
	req, err := a.httpMessageSender.NewCloudEventRequest(a.context)
	if err != nil {
		return err
	}

	a.logger.Info(string(fmt.Sprint(msg.Headers())))

	contentType, err := msgContentType(msg)
	if err != nil {
		return err
	}

	var events []cloudevent.Event
	if contentType == cloudevent.ApplicationCloudEventsBatchJSON {
		events, err = setEventBatchContent(a, msg)
		if err != nil {
			return err
		}
		contentType = cloudevents.ApplicationCloudEventsJSON
	} else {
		event, err := setEventContent(a, msg, contentType)
		events = append(events, event)
		if err != nil {
			return err
		}
	}

	for i, event := range events {
		err = http.WriteRequest(a.context, binding.ToMessage(&event), req)
		if err != nil {
			return err
		}

		res, err := a.httpMessageSender.Send(req)

		if err != nil {
			a.logger.Debug(fmt.Sprintf("Error while sending the message #%d", i+1), zap.Error(err))
			return err
		}

		if res.StatusCode/100 != 2 {
			a.logger.Debug("Unexpected status code", zap.Int("status code", res.StatusCode))
			return fmt.Errorf("%d %s", res.StatusCode, nethttp.StatusText(res.StatusCode))
		}

		reportArgs := &source.ReportArgs{
			Namespace:     a.config.Namespace,
			Name:          a.config.Name,
			ResourceGroup: resourceGroup,
		}

		_ = a.reporter.ReportEventCount(reportArgs, res.StatusCode)
	}

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
