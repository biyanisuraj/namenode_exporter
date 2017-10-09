# namenode_exporter [![Build Status](https://travis-ci.org/timonwong/namenode_exporter.svg)][travis]

[![CircleCI](https://circleci.com/gh/timonwong/namenode_exporter/tree/master.svg?style=shield)][circleci]
[![Docker Repository on Quay](https://quay.io/repository/timonwong/namenode-exporter/status)][quay]
[![Docker Pulls](https://img.shields.io/docker/pulls/timonwong/namenode-exporter.svg?maxAge=604800)][hub]
[![Go Report Card](https://goreportcard.com/badge/github.com/timonwong/namenode_exporter)](https://goreportcard.com/report/github.com/timonwong/namenode_exporter)

**NOTE**: This is a fork of https://github.com/fahlke/namenode_exporter, in addition to the original repo, this repo provides:

* Pre-built binaries
* Docker images

Prometheus exporter for hardware and OS metrics exposed by JXM, written in Go.

## Exported Metrics

| Metric | Meaning | Labels |
| ------ | ------- | ------ |
| namenode_up | Could the namenode be reached | |
| namenode_uptime_seconds | Number of seconds since the namenode started | |
| ... | ... | |

## Building and running

```
make
./namenode_exporter <flags>
```

## Running tests

```
make test
```

## Flags

To see all available configuration flags:

```
./namenode_exporter --help
```

* __`namenode.jmx.url`:__ Namenode JMX URL. (default "http://localhost:50070/jmx")
* __`namenode.jmx.timeout`:__ Timeout reading from namenode JMX url. (default 5s)
* __`namenode.pid-file`:__ Optional path to a file containing the namenode PID for additional metrics.
* __`web.listen-address`:__ Address to listen on for web interface and telemetry. (default ":9779")
* __`web.telemetry-path`:__ Path under which to expose metrics. (default "/metrics")
* __`log.format`:__ Set the log target and format. Example: "logger:syslog?appname=bob&local=7" or "logger:stdout?json=true" (default "logger:stderr")
* __`log.level`:__ Only log messages with the given severity or above. Valid levels: [debug, info, warn, error, fatal]
* __`version`:__ Print version information.

## Useful Queries
TODO(fahlke): Add some useful PromQL queries to showcase the namenode_exporter

## Using Docker

You can deploy this exporter using the Docker image from following registry:

* [DockerHub]\: [timonwong/namenode-exporter](https://registry.hub.docker.com/u/timonwong/namenode-exporter/)
* [Quay.io]\: [timonwong/namenode-exporter](https://quay.io/repository/timonwong/namenode-exporter)

[travis]: https://travis-ci.org/timonwong/namenode_exporter
[circleci]: https://circleci.com/gh/timonwong/namenode_exporter
[quay]: https://quay.io/repository/timonwong/namenode-exporter
[hub]: https://hub.docker.com/r/timonwong/namenode-exporter/
[goreportcard]: https://goreportcard.com/report/github.com/fahlke/namenode_exporter
[DockerHub]: https://hub.docker.com
[Quay.io]: https://quay.io
