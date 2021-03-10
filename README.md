# NSE Webhook Injector
[![Go Report Card](https://goreportcard.com/badge/github.com/gw-tester/nse-injector-webhook)](https://goreportcard.com/report/github.com/gw-tester/nse-injector-webhook)
[![GoDoc](https://godoc.org/github.com/gw-tester/nse-injector-webhook?status.svg)](https://godoc.org/github.com/gw-tester/nse-injector-webhook)
[![Docker](https://images.microbadger.com/badges/image/gwtester/nse-injector-webhook.svg)](http://microbadger.com/images/gwtester/nse-injector-webhook)
[![License](https://img.shields.io/badge/License-Apache%202.0-blue.svg)](https://opensource.org/licenses/Apache-2.0)

## Summary

This project provides a [Kubernetes Mutating Admission Webhook][1]
server that injects [Network Service Mesh Endpoint][2] sidecar.

### Requirements

* Install the latest [NSM services][3].

* The namespace must have enabled the sidecar injection through a label:

```bash
kubectl label namespace default nse-sidecar-injection=enabled
```

* The Kubernetes Pod requires at least one NSM endpoint definition:

```yaml
apiVersion: v1
kind: Pod
metadata:
  name: example
  annotations:
    ns.networkservicemesh.io/endpoints: |
      {
        "name": "lte-network",
        "networkServices": [
          {
            "link": "s5u",
            "labels": "app=pgw-s5u",
            "ipaddress": "172.25.0.0/24"
          },
          {
            "link": "sgi",
            "labels": "app=http-server-sgi",
            "ipaddress": "10.0.1.0/24",
            "route": "10.0.3.0/24"
          }
        ]
      }
spec:
  containers:
    - image: busybox:stable
      name: instance
      command:
        - sleep
      args:
        - infinity
```

[1]: https://kubernetes.io/docs/reference/access-authn-authz/extensible-admission-controllers/
[2]: https://github.com/gw-tester/nse
[3]: https://github.com/networkservicemesh/networkservicemesh
