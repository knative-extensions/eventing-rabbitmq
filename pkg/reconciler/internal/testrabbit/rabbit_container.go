package testrabbit

import (
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"math/rand"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"testing"

	"github.com/docker/go-connections/nat"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/wait"
	"gotest.tools/assert"
	"knative.dev/eventing-rabbitmq/pkg/reconciler/trigger/resources"
)

const RabbitVersion = "3.8"
const RabbitUsername = "guest"
const RabbitPassword = "guest"
const BrokerPort = "5672"
const managementPort = resources.DefaultManagementPort

func AutoStartRabbit(t *testing.T, ctx context.Context) testcontainers.Container {
	rabbitContainer, err := testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{
		ContainerRequest: testcontainers.ContainerRequest{
			Image: fmt.Sprintf("rabbitmq:%s-management-alpine", RabbitVersion),
			Name:  fmt.Sprintf("test-rabbit-%s-%d", t.Name(), rand.Int()),
			ExposedPorts: []string{
				fmt.Sprintf("%s/tcp", BrokerPort),
				fmt.Sprintf("%d/tcp", managementPort),
			},
			WaitingFor: wait.ForLog("Server startup complete"),
		},
		Started: true,
	})
	assert.NilError(t, err)
	return rabbitContainer
}

func TerminateContainer(t *testing.T, ctx context.Context, rabbitContainer testcontainers.Container) {
	assert.NilError(t, rabbitContainer.Terminate(ctx))
}

func BrokerUrl(t *testing.T, ctx context.Context, rabbitContainer testcontainers.Container) string {
	return authenticatedContainerUrl(t, ctx, rabbitContainer, "amqp", RabbitUsername, RabbitPassword, BrokerPort)
}

func ManagementUrl(t *testing.T, ctx context.Context, rabbitContainer testcontainers.Container) string {
	return containerUrl(t, ctx, rabbitContainer, "http", containerManagementPort())
}

func CreateDurableQueue(t *testing.T, ctx context.Context, rabbitContainer testcontainers.Container, queueName string) {
	managementUrl := ManagementUrl(t, ctx, rabbitContainer)
	sendCreationRequest(t, durableQueuePutRequest(t, managementUrl, queueName))
}

func CreateNonDurableQueue(t *testing.T, ctx context.Context, rabbitContainer testcontainers.Container, queueName string) {
	managementUrl := ManagementUrl(t, ctx, rabbitContainer)
	sendCreationRequest(t, queuePutRequest(t, managementUrl, queueName))
}

func CreateExchange(t *testing.T, ctx context.Context, rabbitContainer testcontainers.Container, exchangeName, exchangeType string) {
	managementUrl := ManagementUrl(t, ctx, rabbitContainer)
	sendCreationRequest(t, exchangePutRequest(t, managementUrl, exchangeName, exchangeType))
}

type Binding = map[string]interface{}

func FindBindings(t *testing.T, ctx context.Context, rabbitContainer testcontainers.Container) []Binding {
	managementUrl := ManagementUrl(t, ctx, rabbitContainer)
	response, err := (&http.Client{}).Do(bindingsGetRequest(t, managementUrl))
	assert.NilError(t, err)
	assertOkResponse(t, response)

	var result []Binding
	err = json.Unmarshal(responseBody(t, response), &result)
	assert.NilError(t, err)
	return result
}

type Exchange = map[string]interface{}

func FindOwnedExchanges(t *testing.T, ctx context.Context, rabbitContainer testcontainers.Container) []Exchange {
	managementUrl := ManagementUrl(t, ctx, rabbitContainer)
	response, err := (&http.Client{}).Do(exchangesGetRequest(t, managementUrl))
	assert.NilError(t, err)
	assertOkResponse(t, response)

	var result []Exchange
	err = json.Unmarshal(responseBody(t, response), &result)
	assert.NilError(t, err)
	return keepOwnedExchanges(result)
}

type Queue = map[string]interface{}

func FindQueues(t *testing.T, ctx context.Context, rabbitContainer testcontainers.Container) []Exchange {
	managementUrl := ManagementUrl(t, ctx, rabbitContainer)
	response, err := (&http.Client{}).Do(queuesGetRequest(t, managementUrl))
	assert.NilError(t, err)
	assertOkResponse(t, response)

	var result []Exchange
	err = json.Unmarshal(responseBody(t, response), &result)
	assert.NilError(t, err)
	return keepOwnedExchanges(result)
}

func keepOwnedExchanges(exchanges []Exchange) []Exchange {
	var result []Exchange
	for _, exchange := range exchanges {
		if exchange["user_who_performed_action"] == RabbitUsername {
			result = append(result, exchange)
		}
	}
	return result
}

func Host(t *testing.T, ctx context.Context, rabbitContainer testcontainers.Container) string {
	host, err := rabbitContainer.Host(ctx)
	assert.NilError(t, err)
	return host
}

func ManagementPort(t *testing.T, ctx context.Context, rabbitContainer testcontainers.Container) int {
	mappedPort, err := rabbitContainer.MappedPort(ctx, containerManagementPort())
	assert.NilError(t, err)
	return mappedPort.Int()
}

func containerManagementPort() nat.Port {
	return nat.Port(strconv.Itoa(managementPort))
}

func authenticatedContainerUrl(t *testing.T, ctx context.Context, rabbitContainer testcontainers.Container, protocol, username, password string, port nat.Port) string {
	host := Host(t, ctx, rabbitContainer)
	mappedPort, err := rabbitContainer.MappedPort(ctx, port)
	assert.NilError(t, err)
	return fmt.Sprintf("%s://%s:%s@%s:%s", protocol, username, password, host, mappedPort.Port())
}

func containerUrl(t *testing.T, ctx context.Context, rabbitContainer testcontainers.Container, protocol string, port nat.Port) string {
	host := Host(t, ctx, rabbitContainer)
	mappedPort, err := rabbitContainer.MappedPort(ctx, port)
	assert.NilError(t, err)
	return fmt.Sprintf("%s://%s:%s", protocol, host, mappedPort.Port())
}

func sendCreationRequest(t *testing.T, request *http.Request) {
	response, err := (&http.Client{}).Do(request)
	assert.NilError(t, err)
	assertCreatedResponse(t, response)
}

func durableQueuePutRequest(t *testing.T, brokerUrl string, queueName string) *http.Request {
	return newQueuePutRequest(t, brokerUrl, queueName, `{"auto_delete": false, "durable": false, "arguments": {}}`)
}

func queuePutRequest(t *testing.T, brokerUrl string, queueName string) *http.Request {
	return newQueuePutRequest(t, brokerUrl, queueName, `{"auto_delete": false, "durable": false, "arguments": {}}`)
}

func newQueuePutRequest(t *testing.T, brokerUrl, queueName, requestBody string) *http.Request {
	uri := fmt.Sprintf("%s/api/queues/%s/%s", brokerUrl, url.QueryEscape("/"), url.QueryEscape(queueName))
	request, err := http.NewRequest("PUT", uri, strings.NewReader(requestBody))
	assert.NilError(t, err)
	request.SetBasicAuth(RabbitUsername, RabbitPassword)
	return request
}

func exchangePutRequest(t *testing.T, brokerUrl string, exchangeName, exchangeType string) *http.Request {
	uri := fmt.Sprintf("%s/api/exchanges/%s/%s", brokerUrl, url.QueryEscape("/"), url.QueryEscape(exchangeName))
	request, err := http.NewRequest("PUT", uri, strings.NewReader(
		fmt.Sprintf(`{"type": "%s", "auto_delete": false, "durable": true, "arguments": {}}`, exchangeType)))
	assert.NilError(t, err)
	request.SetBasicAuth(RabbitUsername, RabbitPassword)
	return request
}

func bindingsGetRequest(t *testing.T, brokerUrl string) *http.Request {
	uri := fmt.Sprintf("%s/api/bindings/%s/", brokerUrl, url.QueryEscape("/"))
	return getJsonRabbitRequest(t, uri)
}

func exchangesGetRequest(t *testing.T, brokerUrl string) *http.Request {
	uri := fmt.Sprintf("%s/api/exchanges/%s/", brokerUrl, url.QueryEscape("/"))
	return getJsonRabbitRequest(t, uri)
}

func queuesGetRequest(t *testing.T, brokerUrl string) *http.Request {
	uri := fmt.Sprintf("%s/api/queues/%s/", brokerUrl, url.QueryEscape("/"))
	return getJsonRabbitRequest(t, uri)
}

func getJsonRabbitRequest(t *testing.T, uri string) *http.Request {
	request, err := http.NewRequest("GET", uri, nil)
	assert.NilError(t, err)
	request.SetBasicAuth(RabbitUsername, RabbitPassword)
	request.Header.Set("Accept", "application/json")
	return request
}

func assertCreatedResponse(t *testing.T, response *http.Response) {
	actualStatusCode := response.StatusCode
	assert.Equal(t, 201, actualStatusCode,
		fmt.Sprintf("Expected 201, got %d. HTTP response:\n%s", actualStatusCode, string(responseBody(t, response))))
}

func assertOkResponse(t *testing.T, response *http.Response) {
	actualStatusCode := response.StatusCode
	assert.Equal(t, 200, actualStatusCode, fmt.Sprintf("Expected 200, got %d", actualStatusCode))
}

func responseBody(t *testing.T, response *http.Response) []byte {
	responseBody, err := ioutil.ReadAll(response.Body)
	assert.NilError(t, err)
	return responseBody
}
