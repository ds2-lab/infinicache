## Infinicache Lazy Migration from AWS to K8s

This branch contains a fork of [Infinicache](https://github.com/mason-leap-lab/infinicache) 
which was modified in such a way that we are able to create 2 different containers (1 for the 
proxy and 1 for the cache nodes) and then deploy such components in Kubernetes and Knative. 
A more detailed view about what the repository contains can be found in `/docs`.

#### Install Infinicache in k8s
1) Setup a Google Cloud account (Google offers free credit) and enable Google Kubernetes Engine (GKE) and Google Build.
Follow GCP guidelines carefully to correctly setup `gcloud`.

2) Once you have GKE enabled, you should able to create a cluster, therefore modify file
 `/setup/config` with your desired configuration and from the `proxy/` folder create a GKE
  cluster *without* Istio add-on. We do this because the Istio version of the add-on usually 
  lags behind what Knative expects:

```shell script
$ ./setup/create-gke-cluster
```
3) Install Istio
```shell script
$ ./install-istio
```
4) Install Knative Serving
```shell script
$ ./install-serving
```
5) Install Knative Eventing

```shell script
$ ./install-eventing
```
6) (Optional, but recommended) Install observability features to enable logging, metrics, 
and request tracing in Serving and Eventing components:
```shell script
$ ./install-monitoring
```
7) Check if all pods are up and running:
```shell script
$ kubectl get pods --all-namespaces
```
NB: The repository is configured to work with 20 cache nodes. If you want to modify this 
number you may want to change file `proxy/proxy/server/config.go`.
9) Modify files `node/node-knService.yaml` and `proxy/proxy-deployment.yaml` with the correct iage adress

8) Go to the node module and containerize the application.
```shell script
$ cd node/
$ gcloud builds submit --tag gcr.io/{PROJECT-ID}/infinicache-node
```
9) Deploy 20 local cache nodes using Knative
```shell script
$ cd node/
$ ./createNodes 19
```
10) Find the nodes addresses
```shell script
$ kubectl get ksvc
```
11) Configure the proxy with the nodes addresses. Modify the file 
`proxy/proxy/server/config.go` by modifying the LambdaAddresses array.
12) Go to the proxy module and containerize the application.
```shell script
$ cd proxy/
$ gcloud builds submit --tag gcr.io/{PROJECT-ID}/infinicache-proxy
```
13) Modify file `proxy/proxy-deployment.yaml` with the correct image address
14) Create deployment and service for proxy
```shell script
$ cd proxy/
$ kubectl apply - proxy-service.yaml
$ kubectl apply -f proxy-deployment.yaml
```
15) Wait proxy to become READY
```shell script
$ kubectl get pods --watch
```
16) SSH to Proxy pod
```shell script
$ kubectl exec -it {POD-NAME} /bin/bash
```
17) Launch Proxy
```shell script
$ make start
```
18) Run Test
```shell script
$ go run client/example/main.go
```