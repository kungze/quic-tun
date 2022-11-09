# Restful API

``quic-tun`` provide some restful API. By these APIs, you can query or operate on the tunnel which are active.
You can set address of the API server listen on by ``--httpd-listen-on`` when you start server/client endpoint server, like below:

```console
./quictun-server --httpd-listen-on 127.0.0.1:18086
```

## Get all tunnels information

**Request order:**

```console
curl --location --request GET http://127.0.0.1:18086/tunnels
```

**For example:**

```console
curl --location --request GET http://127.0.0.1:18086/tunnels
```

**Return to information:**

```console
[
  {
    "uuid": "9eb73491-ef38-463d-85c3-d4512152d224",
    "streamId": 0,
    "endpoint": "server",
    "serverAppAddr": "172.18.11.2:5915",
    "remoteEndpointAddr": "172.18.29.161:56465",
    "createdAt": "2022-06-21 11:41:28.85774404 +0800 CST m=+47.535828999",
    "serverTotalBytes": 1545,
    "clientTotalBytes": 2221,
    "serverSendRate": "0.00 kB/s",
    "clientSendRate": "0.00 kB/s",
    "protocol": "spice",
    "protocolProperties": {
      "version": "2.2",
      "sessionId": "d0306d75",
      "channelType": "main",
      "serverName": "instance-e548a827-8937-4047-a756-e56937017128",
      "serverUUID": "e548a827-8937-4047-a756-e56937017128"
    }
  },
  {
    "uuid": "66bad84d-318c-4e14-b3be-a5cb796e7f61",
    "streamId": 44,
    "endpoint": "server",
    "serverAppAddr": "172.18.11.2:5915",
    "remoteEndpointAddr": "172.18.29.161:56465",
    "createdAt": "2022-06-21 11:41:28.937090895 +0800 CST m=+47.615175866",
    "serverTotalBytes": 1545,
    "clientTotalBytes": 2221,
    "serverSendRate": "0.00 kB/s",
    "clientSendRate": "0.00 kB/s",
    "protocol": "spice",
    "protocolProperties": {
      "version": "2.2",
      "sessionId": "d0306d75",
      "channelType": "record"
    }
  }
  ...
 ]

```

## Close the tunnel according to uuid

**Request order:**

```console
curl --location --request PUT 'http://127.0.0.1:18086/<uuid>/close_tunnel'
```

**For example:**

```console
curl --location --request PUT 'http://127.0.0.1:18086/66bad84d-318c-4e14-b3be-a5cb796e7f61/close_tunnel'
```

**Return to information:**

No return value for successful closure, but the status code is 200
