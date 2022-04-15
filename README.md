# quic-tun

[![Release][1]][2] [![MIT licensed][3]][4]

[1]: https://img.shields.io/github/v/release/jeffyjf/quic-tun?color=orange
[2]: https://github.com/jeffyjf/quic-tun/releases/latest
[3]: https://img.shields.io/github/license/jeffyjf/quic-tun
[4]: LICENSE


Establish a fast&security tunnel, make you can access remote TCP/UNIX application like local application.


## QuickStart

Increase the maximum buffer size, read the
[docs](https://github.com/lucas-clemente/quic-go/wiki/UDP-Receive-Buffer-Size) for details

```
sysctl -w net.core.rmem_max=2500000
```

Download a corresponding one from precompiled from [releases](https://github.com/jeffyjf/quic-tun/releases) and decompression it.

```
wget https://github.com/jeffyjf/quic-tun/releases/download/v0.0.1/quic-tun_0.0.1_linux_amd64.tar.gz
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
./quictun-client --listen-on tcp:127.0.0.1:6500 --server-endpoint 172.18.31.36:7500 --token 172.18.30.117:22
```

**Note:** The parameter specified by `--token` used to tell `quictun-server` the application address that the client want to access.

Use `ssh` command to test

```
$ ssh root@127.0.0.1 -p 6500
root@127.0.0.1's password:
```
