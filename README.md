# Overview

Proxy client could connect to NSMgr by provided interface and perform all required activities providing a proxy host interface.

Every request will be full-filled with a proper spire token.

# Build

## Build cmd-nsc-proxy binary locally

You can build the locally by executing

```bash
go build ./...
```

## Build Docker container

You can build the docker container by running:

```bash
docker build .
```

# Testing

## Testing Docker container

Testing is run via a Docker container.  To run testing run:

```bash
docker run --rm $(docker build -q --target test .)
```

# Debugging

## Debugging the tests
If you wish to debug the test code itself, that can be achieved by running:

```bash
docker run --rm -p 40000:40000 $(docker build -q --target debug .)
```

This will result in the tests running under dlv.  Connecting your debugger to localhost:40000 will allow you to debug.

```bash
-p 40000:40000
```
forwards port 40000 in the container to localhost:40000 where you can attach with your debugger.

```bash
--target debug
```

Runs the debug target, which is just like the test target, but starts tests with dlv listening on port 40000 inside the container.

## Debugging the cmd-nsc-proxy

When you run 'cmd-nsc-proxy' you will see an early line of output that tells you:

```Setting env variable DLV_LISTEN_FORWARDER to a valid dlv '--listen' value will cause the dlv debugger to execute this binary and listen as directed.```

If you follow those instructions when running the Docker container:
```bash
docker run -e DLV_LISTEN_NSMGR=:50000 -p 50000:50000 --rm $(docker build -q --target test .)
```

```-e DLV_LISTEN_NSMGR=:50000``` tells docker to set the environment variable DLV_LISTEN_NSMGR to :50000 telling
dlv to listen on port 50000.

```-p 50000:50000``` tells docker to forward port 50000 in the container to port 50000 in the host.  From there, you can
just connect dlv using your favorite IDE and debug cmd-nsc-proxy.

## Debugging the tests and the cmd-nsc-proxy

```bash
docker run --rm -p 40000:40000 $(docker build -q --target debug .)
```

Please note, the tests **start** the cmd-nsc-proxy, so until you connect to port 40000 with your debugger and walk the tests
through to the point of running cmd-nsc-proxy, you will not be able to attach a debugger on port 50000 to the cmd-nsc-proxy.

# Build Docker image compatible with integration testing suite: 

`docker build . -t networkservicemeshci/cmd-cmd-nsc-proxy:master && kind load docker-image networkservicemeshci/cmd-cmd-nsc-proxy:master`   
