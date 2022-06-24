# Network Latency Test

In the test scenario, we will have some public network test cases, in these test
cases we place the server machine in a remoter location.

## Preparation

* In server machine

  Start TCP service

  ```console
  $ ./server
  The server listen on 0.0.0.0:15676
  ```

  Open a new terminal, start `quictun-server`:

    ```console
  $ ./quictun-server
  I0624 09:15:29.223140    1515 server.go:30] "Server endpoint start up successful" listen address="[::]:7500"
  ```

* In client machine

  Start `quictun-client`

  ```console
  $ ./quictun-client --server-endpoint 192.168.26.129:7500 --token-source tcp:127.0.0.1:5201 --insecure-skip-verify true
  I0624 09:17:30.926905    1679 client.go:35] "Client endpoint start up successful" listen address="127.0.0.1:6500"
  ```

  The `192.168.26.129` is the server machine's IP address, in public network test cases this is a public IP address.

  Open a new terminal, run `client` python script to test network latency.

  * Test the performance by TCP directly:

    ```console
    ./client --server-host 192.168.26.131
    ```

  * Test the performance by `quic-tun` forward the traffic:

    ```console
    ./client --server-host 127.0.0.1 --server-port 6500
    ```

Please refer to the [doc](../../tests/latency/) to learn more informations about test tools `server` and `client`.

## Test results

### Packet loss rate: 0.0% (LAN)

* TCP

```console
$ ./client --server-host 192.168.26.131
First packet latency: 0.7572174072265625 ms
Total latency: 499.89843368530273 ms
```

* quic-tun

```console
$ ./client --server-host 127.0.0.1 --server-port 6500
First packet latency: 7.899284362792969 ms
Total latency: 591.1564826965332 ms
```

### Packet loss rate: 1.0% (LAN)

* TCP

```console
$ ./client --server-host 192.168.26.131
First packet latency: 0.6091594696044922 ms
Total latency: 4290.04430770874 ms
```

* quic-tun

```console
$ ./client --server-host 127.0.0.1 --server-port 6500
First packet latency: 8.286714553833008 ms
Total latency: 1201.0939121246338 ms
```

### Packet loss rate: 0.0% (WAN)

* TCP

```console
$ ./client --server-host 47.111.149.1 --server-port 5201
First packet latency: 24.95884895324707 ms
Total latency: 25493.980407714844 ms
```

* quic-tun

```console
$ ./client --server-host 127.0.0.1 --server-port 6500
First packet latency: 100.67296028137207 ms
Total latency: 24987.539291381836 ms
```

### Packet loss rate: 1.0% (WAN)

* TCP

```console
$ ./client --server-host 47.111.149.1 --server-port 5201
First packet latency: 23.789167404174805 ms
Total latency: 28489.194869995117 ms
```

* quic-tun

```console
$ ./client --server-host 127.0.0.1 --server-port 6500
First packet latency: 103.04689407348633 ms
Total latency: 27247.18403816223 ms
```

### Packet loss rate: 2.0% (WAN)

* TCP

```console
$ ./client --server-host 47.111.149.1 --server-port 5201
First packet latency: 36.420583724975586 ms
Total latency: 33720.38745880127 ms
```

* quic-tun

```console
$ ./client --server-host 127.0.0.1 --server-port 6500
First packet latency: 100.87323188781738 ms
Total latency: 27528.08117866516 ms
```
