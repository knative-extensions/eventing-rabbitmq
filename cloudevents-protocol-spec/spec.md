# RabbitMQ Protocol Binding for CloudEvents - Version 1.0.2-wip
## Abstract

The RabbitMQ Binding for CloudEvents defines how events are mapped to [RabbitMQ messages][rabbit-msg].

## Status of this document

This document is a working draft.

## Table of Contents

1. [Introduction](#1-introduction)

- 1.1. [Conformance](#11-conformance)
- 1.2. [Relation to RabbitMQ](#12-relation-to-rabbitmq)
- 1.3. [Content Modes](#13-content-modes)
- 1.4. [Event Formats](#14-event-formats)
- 1.5. [Security](#15-security)

2. [Use of CloudEvents Attributes](#2-use-of-cloudevents-attributes)

3. [RabbitMQ Message Mapping](#3-rabbitmq-message-mapping)

- 3.1. [Binary Content Mode](#31-binary-content-mode)
- 3.2. [Structured Content Mode](#32-structured-content-mode)

4. [References](#4-references)

## 1. Introduction

[CloudEvents][ce] is a standardized and protocol-agnostic definition of the structure and metadata description of events. This specification defines how the elements defined in the CloudEvents specification are to be used in [RabbitMQ][rabbitmq] as [RabbitMQ messages][rabbit-msg].

### 1.1. Conformance

The key words "MUST", "MUST NOT", "REQUIRED", "SHALL", "SHALL NOT", "SHOULD", "SHOULD NOT", "RECOMMENDED", "MAY", and "OPTIONAL" in this document are to be interpreted as described in [RFC2119][rfc2119].

### 1.2. Relation to RabbitMQ

This specification does not prescribe rules constraining transfer or settlement of event messages with RabbitMQ; it solely defines how CloudEvents are expressed as [RabbitMQ messages][rabbit-msg].

[RabbitMQ supports multiple messaging protocols][rabbitmq-protocols] - AMQP 1.0, AMQP 0-9-1, STOMP, MQTT - and uses [AMQP 0-9-1][amqp091] with extensions as the "core" & default protocol. AMQP 0-9-1 is a binary protocol with strong messaging semantics with [wide client library support][rabbitmq-clients].

This binding specification defines how attributes and data of a CloudEvent is mapped to the data and headers sections of a RabbitMQ message.

This specification assumes use of the default [RabbitMQ message properties][rabbit-msg].



### 1.3. Content Modes

This specification defines two content modes for transferring events: _binary_ and _structured_. Every compliant implementation SHOULD support the _structured_ and _binary_ modes.

In the _structured_ content mode, event metadata attributes and event data are placed into the RabbitMQ message's content body section using an event format as defined in the CloudEvents [spec][ce].

In the _binary_ content mode, the value of the event `data` is placed into the RabbitMQ message's content body section as-is, with the `datacontenttype` attribute value declaring its media type mapped to the RabbitMQ `content-type` message property; all other event attributes are mapped to the [RabbitMQ message headers][rabbit-msg].

### 1.4. Event Formats

Event formats, used with the _structured_ content mode, define how an event is expressed in a particular data format. All implementations of this specification that support the _structured_ content mode MUST support the [JSON event format][json-format].

### 1.5. Security

This specification does not introduce any new security features for RabbitMQ, or mandate specific existing features to be used.

## 2. Use of CloudEvents Attributes

This specification does not further define any of the [CloudEvents][ce] event attributes.

### 2.1. data

`data` is assumed to contain opaque application data that is
encoded as declared by the `datacontenttype` attribute.

An application is free to hold the information in any in-memory representation
of its choosing, but as the value is transposed into RabbitMQ as defined in this
specification, RabbitMQ delivery provides data available as a sequence of bytes.

For instance, if the declared `datacontenttype` is
`application/json;charset=utf-8`, the expectation is that the `data`
value is made available as [UTF-8][rfc3629] encoded JSON text.


## 3. RabbitMQ Message Mapping

The content mode is chosen by the producer of the event. Protocol interaction patterns that might allow solicitation of events using a particular content mode (explained below) might be defined by an application, but are not defined here.

The receiver of the event can distinguish between the two modes by inspecting the `content-type` message property field. If the value is prefixed with the CloudEvents media type `application/cloudevents`, indicating the use of a known [event format](#14-event-formats), the receiver uses _structured_ mode, otherwise it defaults to _binary_ mode.

If a receiver detects the CloudEvents media type, but with an event format that it cannot handle, for instance `application/cloudevents+avro`, it MAY still treat the event as binary and forward it to another party as-is.

When the `content-type` message property is not prefixed with the CloudEvents media type, being able to know when the message ought to be attempted to be parsed as a CloudEvent can be a challenge. While this specification can not mandate that senders do not include any of the CloudEvents message properties when the message is not a CloudEvent, it would be reasonable for a receiver to assume that if the message has all of the mandatory CloudEvents attributes as message properties then it's probably a CloudEvent. However, as with all CloudEvent messages, if it does not adhere to all of the normative language of this specification then it is not a valid CloudEvent.

### 3.1. Binary Content Mode

The _binary_ content mode accommodates any shape of event data, and allows for efficient transfer and without transcoding effort.

#### 3.1.1. RabbitMQ Content-Type

For the _binary_ mode, the RabbitMQ `content-type` property field value MUST be mapped to the CloudEvents `datacontenttype` attribute.

#### 3.1.2. Event Data Encoding

The [`data`](#21-data) byte-sequence MUST be used as the value of the RabbitMQ message.

#### 3.1.3. Metadata Headers

All [CloudEvents][ce] attributes with exception of `datacontenttype` and `data` MUST be individually mapped to and from the [RabbitMQ message properties][rabbit-msg] section.

CloudEvents extensions that define their own attributes MAY define a secondary mapping to RabbitMQ message properties for those attributes, also in different message sections, especially if specific attributes or their names need to align with RabbitMQ features or with other specifications that have explicit RabbitMQ message header bindings. However, they MUST also include the previously defined primary mapping.

An extension specification that defines a secondary mapping rule for RabbitMQ, and any revision of such a specification, MUST also define explicit mapping rules for all other protocol bindings that are part of the CloudEvents core at the time of the submission or revision.

##### 3.1.3.1. RabbitMQ Application Property Names

CloudEvent properties in _structured_ mode are mapped “as is” into the content data field as key:value pairs.

CloudEvent properties in _binary_ mode are prefixed with "ce-" to use in the content headers section

Examples:

    * `time` maps to `ce-time`
    * `id` maps to `ce-id`
    * `specversion` maps to `ce-specversion`

##### 3.1.3.2. RabbitMQ Application Property Values

The value for each RabbitMQ header is constructed from the respective RabbitMQ representation, compliant with the [RabbitMQ message properties][rabbit-msg] specification.

### 3.2. Structured Content Mode

The _structured_ content mode keeps event metadata and data together in the payload, allowing simple forwarding of the same event across multiple routing hops, and across multiple protocols.

#### 3.2.1. RabbitMQ Content-Type

The [RabbitMQ `content-type`][content-type] property field is set to the media type of an [event format](#14-event-formats).

Example for the [JSON format][json-format]:

```text
content-type: application/cloudevents+json
```
#### 3.2.2. Event Data Encoding

The chosen [event format](#14-event-formats) defines how all attributes and `data` are represented.

The event metadata and data is then rendered in accordance with the event format specification and the resulting data becomes the RabbitMQ content body section.

#### 3.2.3. Metadata Headers

Implementations MAY include the same RabbitMQ application-properties as defined for the [binary mode](#313-metadata-headers).

#### 3.2.4 Examples

This example shows a JSON event format encoded event:

```text
--------------- headers ----------------------------------
{
    content-type: application/cloudevents+json; charset=utf-8
}
--------------- body -------------------------------------

{
    “specversion” : "1.0",
    “id”: “12341234”
    “type” : "com.example.someevent",
    “datacontenttype”: “application/xml; charset=utf-8”

    ... further attributes omitted ...
    “data”: {

        ... application data encoded in XML ...

    }
}

----------------------------------------------------------
```
This example shows a Binary event format encoded event:

```text
--------------- headers -----------------------------------
{
    ce-specversion : "1.0",
    ce-id: “12341234”
    ce-type: "com.example.someevent",
    ce-source: "example/source.uri"
    ce-extension: "test extension value",
    ce-datacontenttype: "application/avro; charset=UTF-8"

    ... further attributes omitted ...
}
--------------- body --------------------------------------

      ... application data encoded in Avro ...

-----------------------------------------------------------
```

## 4. References

- [RabbitMQ][rabbitmq] RabbitMQ message protocol
- [AMQP 1.0][amqp1] AMQP 1.0 protocol binding spec for Cloud Events
- [AMQP 0-9-1][amqp091] AMQP 0-9-1 Spec document
- [RFC2046][rfc2046] Multipurpose Internet Mail Extensions (MIME) Part Two:
  Media Types
- [RFC2119][rfc2119] Key words for use in RFCs to Indicate Requirement Levels
- [RFC3629][rfc3629] UTF-8, a transformation format of ISO 10646
- [RFC4627][rfc4627] The application/json Media Type for JavaScript Object
  Notation (JSON)
- [RFC6839][rfc6839] Additional Media Type Structured Syntax Suffixes
- [RFC7159][rfc7159] The JavaScript Object Notation (JSON) Data Interchange
  Format

[rabbitmq]: https://rabbitmq.com/amqp-0-9-1-reference.html
[rabbit-msg]: https://www.rabbitmq.com/publishers.html#message-properties
[amqp1]: https://github.com/cloudevents/spec/blob/master/amqp-protocol-binding.md
[amqp091]: https://www.rabbitmq.com/resources/specs/amqp0-9-1.pdf
[rabbitmq-protocols]: https://www.rabbitmq.com/protocols.html
[rabbitmq-clients]: https://www.rabbitmq.com/devtools.html
[ce]: https://github.com/cloudevents/spec/blob/v1.0.1/spec.md
[json-format]: https://github.com/cloudevents/spec/blob/master/json-format.md
[content-type]: https://tools.ietf.org/html/rfc7231#section-3.1.1.5
[json-value]: https://tools.ietf.org/html/rfc7159#section-3
[rfc2046]: https://tools.ietf.org/html/rfc2046
[rfc2119]: https://tools.ietf.org/html/rfc2119
[rfc3629]: https://tools.ietf.org/html/rfc3629
[rfc4627]: https://tools.ietf.org/html/rfc4627
[rfc6839]: https://tools.ietf.org/html/rfc6839
[rfc7159]: https://tools.ietf.org/html/rfc7159
