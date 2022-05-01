# System V Wrapper

![](https://github.com/sgilson/go-sysv/actions/workflows/ci.yml/badge.svg)

## Overview

A limited wrapper around [System V inter-process communication facilities][sysv].
This package does not aim to expose the full System V feature set but
rather expose a limited set of features in idiomatic and safe Go.

Implemented facilities:

- [x] Message queues
    - [x] Create queue
    - [x] Delete queue
    - [x] Send
    - [x] Receive
    - [ ] Manage existing queue
- [ ] Semaphore sets
- [ ] Shared memory segments

## Example

The example files can be run using the following commands:

```shell
go run example/sender/main.go    # Create queue and send message
go run example/receiver/main.go  # Receive one message
go run example/cleanup/main.go   # Delete queue from system
```

Send and receive can be run in any order, though note the receiver
will block until a message or interrupt arrives.

If you experience trouble on your system and need to delete queues without
using this package, use `ipcs` and `ipcrm`.

```shell
# Show all system v resources
ipcs -oa
```

```shell
# Delete queue by id
ipcrm -q $QUEUE_ID
```

## Installation

Install this package using `go get`:

```shell
go get github.com/sgilson/go-sysv
```

[sysv]: https://man7.org/linux/man-pages/man7/sysvipc.7.html