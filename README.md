# RabbitMQ Eventing components

**[These components are BETA](https://github.com/knative/community/tree/main/mechanics/MATURITY-LEVELS.md)**

[![go.dev reference](https://img.shields.io/badge/go.dev-reference-007d9c?logo=go&logoColor=white)](https://pkg.go.dev/knative.dev/eventing-rabbitmq)
[![Go Report Card](https://goreportcard.com/badge/knative-sandbox/eventing-rabbitmq)](https://goreportcard.com/report/knative-sandbox/eventing-rabbitmq)
[![Releases](https://img.shields.io/github/release-pre/knative-sandbox/eventing-rabbitmq.svg)](https://github.com/knative-sandbox/eventing-rabbitmq/releases)
[![LICENSE](https://img.shields.io/github/license/knative-sandbox/eventing-rabbitmq.svg)](https://github.com/-sandbox/eventing-rabbitmq/blob/master/LICENSE)
[![codecov](https://codecov.io/gh/knative-sandbox/eventing-rabbitmq/branch/master/graph/badge.svg)](https://codecov.io/gh/knative-sandbox/eventing-rabbitmq)
[![TestGrid](https://img.shields.io/badge/testgrid-eventing_rabbitmq-informational?logo=data:image/x-icon;base64,AAABAAEAICAAAAEACACoCAAAFgAAACgAAAAgAAAAQAAAAAEACAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAALAAAAFQAAACoAAw0AAAAAVQAGGgAAAABgAAAAgAAKJgAAAACqAA0zAAATTAAAGmYAAB1zAAAmmQAAM8wAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAACgoKCgoKCAsQEBAQEBANDRAQEBAQEAsPEBAQEBAQAAAKCgoKCgoICxAQEBAQEA0NEBAQEBAQCw8QEBAQEBAAAAoKCgoKCggLEBAQEBAQDQ0QEBAQEBALDxAQEBAQEAAACgoKCgoKCAsQEBAQEBANDRAQEBAQEAsPEBAQEBAQAAAKCgoKCgoICxAQEBAQEA0NEBAQEBAQCw8QEBAQEBAAAAoKCgoKCggLEBAQEBAQDQ0QEBAQEBALDxAQEBAQEAAACAgICAgIBwkPDw8PDw8MDA8PDw8PDwkODw8PDw8PAAALCwsLCwsJBAsLCwsLCwYCAwMDAwMDAQkLCwsLCwsAABAQEBAQEA8LEBAQEBAQDQUKCgoKCgoDDxAQEBAQEAAAEBAQEBAQDwsQEBAQEBANBQoKCgoKCgMPEBAQEBAQAAAQEBAQEBAPCxAQEBAQEA0FCgoKCgoKAw8QEBAQEBAAABAQEBAQEA8LEBAQEBAQDQUKCgoKCgoDDxAQEBAQEAAAEBAQEBAQDwsQEBAQEBANBQoKCgoKCgMPEBAQEBAQAAAQEBAQEBAPCxAQEBAQEA0FCgoKCgoKAw8QEBAQEBAAAA0NDQ0NDQwGDQ0NDQ0NCwMFBQUFBQUCDA0NDQ0NDQAADQ0NDQ0NDAIFBQUFBQUDCw0NDQ0NDQYMDQ0NDQ0NAAAQEBAQEBAPAwoKCgoKCgUNEBAQEBAQCw8QEBAQEBAAABAQEBAQEA8DCgoKCgoKBQ0QEBAQEBALDxAQEBAQEAAAEBAQEBAQDwMKCgoKCgoFDRAQEBAQEAsPEBAQEBAQAAAQEBAQEBAPAwoKCgoKCgUNEBAQEBAQCw8QEBAQEBAAABAQEBAQEA8DCgoKCgoKBQ0QEBAQEBALDxAQEBAQEAAAEBAQEBAQDwMKCgoKCgoFDRAQEBAQEAsPEBAQEBAQAAALCwsLCwsJAQMDAwMDAwIGCwsLCwsLBAkLCwsLCwsAAA8PDw8PDw4JDw8PDw8PDAwPDw8PDw8JBwgICAgICAAAEBAQEBAQDwsQEBAQEBANDRAQEBAQEAsICgoKCgoKAAAQEBAQEBAPCxAQEBAQEA0NEBAQEBAQCwgKCgoKCgoAABAQEBAQEA8LEBAQEBAQDQ0QEBAQEBALCAoKCgoKCgAAEBAQEBAQDwsQEBAQEBANDRAQEBAQEAsICgoKCgoKAAAQEBAQEBAPCxAQEBAQEA0NEBAQEBAQCwgKCgoKCgoAABAQEBAQEA8LEBAQEBAQDQ0QEBAQEBALCAoKCgoKCgAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA)](https://testgrid.knative.dev/eventing-rabbitmq)
[![Slack](https://img.shields.io/badge/Signup-Knative_Slack-white.svg?logo=slack)](https://slack.knative.dev)
[![Slack](https://img.shields.io/badge/%23eventing-white.svg?logo=slack&color=522a5e)](https://knative.slack.com/archives/C9JP909F0)

This repository contains the RabbitMQ Knative Eventing Broker and Source implementations. 

## Documentation

You can check out the [documentation here](docs/README.md). 

## Community 

---

This repo falls under the
[Knative Code of Conduct](https://github.com/knative/community/blob/master/CODE-OF-CONDUCT.md)
