# grpc-tunnel

grpc-tunnel provides an example implementation for a client, server and target of the grpc-tunnel framework.

app (ssh, gnmi, etc) - target grpctunnel <------> server grpctunnel <----> client grpctunnel with e.g. ssh-client

[grpc-tunnel architecture](https://github.com/openconfig/reference/blob/master/rpc/gnmi/gnmignoissh-dialout-grpctunnel.md#target-registration)
[grpc-tunnel design](https://github.com/openconfig/grpctunnel/blob/master/doc/grpctunnel_design.md#message-flow)


## Features

### Server

- act as a registration server for target ID and target Type

### Target

- currently a single tunnel per target list but can be extended
- target names are provided as a string for now
- target types are provided as a string for now

### client

- connects to a single grpc tunnel server and can be embedded in a proxy command

## Installation

install via the following command

```
sudo curl -sL https://raw.githubusercontent.com/henderiw/grpc-tunnel/master/get.sh | sudo bash
```

Upgrades are handled using grpctunnel version upgrade

```
grpctunnel version upgrade
```

## Setup

### generate a certificate for the server

### server


