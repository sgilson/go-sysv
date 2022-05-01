# System V Wrapper

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

[sysv]: https://man7.org/linux/man-pages/man7/sysvipc.7.html