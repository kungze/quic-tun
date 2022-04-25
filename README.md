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
./quictun-client --listen-on tcp:127.0.0.1:6500 --server-endpoint 172.18.31.36:7500 --token 172.18.30.117:22 --insecure-skip-verify True
```

**Note:** The value specified by `--token` used to tell `quictun-server` the application address that the client want to access.

Use `ssh` command to test

```
$ ssh root@127.0.0.1 -p 6500
root@127.0.0.1's password:
```
