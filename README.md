# quic-tun

[![Release][1]][2] [![MIT licensed][3]][4]

[1]: https://img.shields.io/github/v/release/kungze/quic-tun?color=orange
[2]: https://github.com/kungze/quic-tun/releases/latest
[3]: https://img.shields.io/github/license/kungze/quic-tun
[4]: LICENSE


Establish a fast&security tunnel, make you can access remote TCP/UNIX
application like local application.

## Overview

``Quic-tun`` contains two command tools: ``quictun-server`` and ``quictun-client``,
``quictun-server`` used to translate application's transport layer protocol from
TCP/UNIX to [QUIC](https://en.wikipedia.org/wiki/QUIC), and ``quictun-client``
translate back to TCP/UNIX protocol at client side. The schematic diagram like below:

<img src="quic-tun.png" alt="quic-tun"/>

### Concerned issues

If you are hesitating whether or not to study this document deeply or to play
attention to this project. Maybe the below issues will help you to make your decision.

* Whether you often encounter packet loss or high latency issues? And this issues influenced
  your application's performance or stabililty.
* You have multiple applications and listen on multiple ports, but you don't want expose
  too much ports to internet.
* Your application don't support TLS but the security issue is your concerned thing.
* Your application listen on local UNIX socket, but you need to access it at other machines.

If you encounter one or more above scenarios. Congratulations, you find the correct place!

## QuickStart

Increase the maximum buffer size, read the
[docs](https://github.com/lucas-clemente/quic-go/wiki/UDP-Receive-Buffer-Size) for details

```
sysctl -w net.core.rmem_max=2500000
```

Download a corresponding one from precompiled from [releases](https://github.com/kungze/quic-tun/releases) and decompression it.

```
wget https://github.com/kungze/quic-tun/releases/download/v0.0.1/quic-tun_0.0.1_linux_amd64.tar.gz
```

```
tar xvfz quic-tun_0.0.1_linux_amd64.tar.gz
```

Start up server side endpoint

```
./quictun-server --listen-on 172.18.31.36:7500
```

Start up client side endpoint

```
./quictun-client --listen-on tcp:127.0.0.1:6500 --server-endpoint 172.18.31.36:7500 --token-source tcp:172.18.30.117:22 --insecure-skip-verify True
```

**Note:** The value specified by `--token-source` used to tell `quictun-server` the application address that the client want to access.

Use `ssh` command to test

```
$ ssh root@127.0.0.1 -p 6500
root@127.0.0.1's password:
```


## Concepts

* **client endpoint:** A service run on client side, used to accept the client applications' connection request and convert the transport layer protocol from TCP/UNIX-SOCKET to QUIC.
* **server endpoint:** A service run on server side, used to accept the data from client endpoint and forward these data to server application by TCP/UNIX-SOCKET protocol.
* **token:** When a client endpoint receive a new connection request, the client endpoint will retrieve a token according to the request's source address and send the token to server endpoint, the server endpoint will parse and verify the token and get the server application socket address from parsed result. ``quic-tun`` provide multiple type token plugin in order to adapt different use cases.


## Token plugin

### quictun-client

At client side, We address the token plugin as token source plugin, related command options ``--token-source-plugin``, ``--token-source``. Currently, ``quic-tun`` provide two type token source plugin: ``Fixed`` and ``File``.

#### Fixed

``Fixed`` token source plugin always provide one same token, this mean that all of client applications just only connect to one fixed server application.

Example:

```
./quictun-client --listen-on tcp:127.0.0.1:6500 --server-endpoint 172.18.31.36:7500 --token-source-plugin Fixed --token-source tcp:172.18.30.117:22 --insecure-skip-verify True
```

### File

``File`` token source plugin will read token from a file and return different token according to the client application's source address. The file path specified by ``--token-source``.

The file's contents like below:

```
172.26.106.191 tcp:10.20.30.5:2256
172.26.106.192 tcp:10.20.30.6:3306
172.26.106.193 tcp:10.20.30.6:3306
```

The first column are the client application's IP addresses, the second column are the token(The server application's socket addresses which the client application want to access.)

Example:

```
./quictun-client --insecure-skip-verify --server-endpoint 127.0.0.1:7500 --token-source-plugin File --token-source /etc/quictun/tokenfile --listen-on tcp:172.18.31.36:6622
```

### quictun-server

At server side, we address the token plugin as token parser plugin, it used to parse and verify the token and get the server application socket address from the parse result, related command option ``--token-parser-plugin``, ``--token-parser-key``. Currently, ``quic-tun`` just provide one token parser plugin: ``Cleartext``.

#### Cleartext

``Cleartext`` token parser plugin require the token mustn't be encrypted. But you can use ``base64`` to encode token.

Example:

If the client endpoint token is not encoded.

```
./quictun-server --listen-on 172.18.31.36:7500 --token-parser-plugin Cleartext
```

If the client endpoint token is encoded by ``base64``

```
./quictun-server --listen-on 172.18.31.36:7500 --token-parser-plugin Cleartext --token-parser-key base64
```
