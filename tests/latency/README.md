# Network Latency test tool

The directory contain two python scripts: `server` and `client`, these used to test network latency of TCP.
The `server` script will start a TCP service, the `client` script will connect to the TCP service and send
some packets to the service and the service will send the received data to client. The `client` script will
compute the total time that this process cost and use it as the network latency.

## Qickstart

```shell
git clone https://github.com/kungze/quic-tun.git
cd quic-tun/test/latency
```

In server machine, start TCP service by `server` script:

```console
$ ./server
The server listen on 0.0.0.0:15676
```

You can used command `./server --help` to learn more useage methods.

In client machine, use `client` script to connect above TCP service:

```console
$ ./client --server-host 127.0.0.1
First packet latency: 0.30732154846191406 ms
Total latency: 39.47257995605469 ms
```

The `client` script printed the network latency informations as above.
