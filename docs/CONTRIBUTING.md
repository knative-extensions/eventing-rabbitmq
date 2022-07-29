# Contribution guidelines

Glad to have you onboard on Eventing RabbitMQ! Please refer to Knative's overall
[contribution guidelines](https://www.knative.dev/contributing/) to find out how you can help.

## Getting started

1. [Create and checkout a repo fork](#checkout-your-fork)

### Requirements

You need to install:

- [`ko`](https://github.com/google/ko)
- [`docker`](https://www.docker.com/)
- [`Go`](https://golang.org/)
    - check
      [go \<version\>](https://github.com/knative-sandbox/eventing-rabbitmq/blob/master/go.mod)
      for the required Go version used in this project

If a specific version of a requirement is not explicitly defined above, any version will work during development.

### Checkout your fork

To check out this repository:

1. Create your own [fork of this repository](https://help.github.com/articles/fork-a-repo/):
2. Clone it to your machine:

```shell
git clone git@github.com:${YOUR_GITHUB_USERNAME}/eventing-rabbitmq.git
cd eventing-rabbitmq
git remote add upstream https://github.com/knative-sandbox/eventing-rabbitmq.git
git remote set-url --push upstream no_push
```

_Adding the `upstream` remote sets you up nicely for regularly
[syncing your fork](https://help.github.com/articles/syncing-a-fork/)._

Once you reach this point you are ready to do a full build and deploy as follows.

- [Development](DEVELOPMENT.md)
