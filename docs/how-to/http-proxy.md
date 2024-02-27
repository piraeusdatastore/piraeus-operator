# How to Use Piraeus Datastore with an HTTP Proxy

This guide shows you how to configure the DRBD® Module Loader when using a HTTP Proxy.

To complete this guide, you should be familiar with:

* editing `LinstorSatelliteConfiguration` resources.
* using the `kubectl` command line tool to access the Kubernetes cluster.

## Configuration

We will use environment variables to configure the proxy, this tells the drbd-module-loader component to use the proxy for outgoing communication.

Configure the sample below according to your environment and apply the configuration using `kubectl apply -f filename.yml`.

This sample configuration assumes that a HTTP proxy is reacheable at `http://10.0.0.1:3128`.

```yaml
apiVersion: piraeus.io/v1
kind: LinstorSatelliteConfiguration
metadata:
  name: http-proxy
spec:
  podTemplate:
    spec:
      initContainers:
        - name: drbd-module-loader
          env:
            - name: HTTP_PROXY
              value: http://10.0.0.1:3128 # Add your proxy connection here
            - name: HTTPS_PROXY
              value: http://10.0.0.1:3128 # Add your proxy connection here
            - name: NO_PROXY
              value: localhost,127.0.0.1,10.0.0.0/8,172.16.0.0/12 # Add internal IP ranges and domains here
```
