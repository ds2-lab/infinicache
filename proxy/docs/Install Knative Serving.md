#### Installing Knative Serving

- Install the Custom Resource Definitions (aka CRDs):
```bash
$ kubectl apply --filename https://github.com/knative/serving/releases/download/v0.13.0/serving-crds.yaml
```

- Install the core components of Serving
```bash
$ kubectl apply --filename https://github.com/knative/serving/releases/download/v0.13.0/serving-core.yaml
```

- Install the Networking Layer - we choose Ambassador this time, because it seems easier to install
    * Create a namespace to install Ambassador in:
    ```bash
    $ kubectl create namespace ambassador
    ```
    * Install Ambassador
    ```bash
    $ kubectl apply --namespace ambassador \
      --filename https://getambassador.io/yaml/ambassador/ambassador-rbac.yaml \
      --filename https://getambassador.io/yaml/ambassador/ambassador-service.yaml
    ```
    * Give Ambassador the required permissions:
    ```bash
    $ kubectl patch clusterrolebinding ambassador -p '{"subjects":[{"kind": "ServiceAccount", "name": "ambassador", "namespace": "ambassador"}]}'
    ```
    * Enable Knative support in Ambassador
    ```bash
    $ kubectl set env --namespace ambassador  deployments/ambassador AMBASSADOR_KNATIVE_SUPPORT=true  
    ```
    * Configure Knative Serving to use Ambassador by default
    ```bash
    $ kubectl patch configmap/config-network \
      --namespace knative-serving \
      --type merge \
      --patch '{"data":{"ingress.class":"ambassador.ingress.networking.knative.dev"}}'
    ```
    * Fetch External IP or CNAME (to be used to configure DNS below)
    ```bash
    $ kubectl --namespace ambassador get service ambassador
    ``` 
 - Configure DNS: We ship a simple Kubernetes Job called “default domain” that will 
 (see caveats) configure Knative Serving to use xip.io as the default DNS suffix. 
 ```bash
 $ kubectl apply --filename https://github.com/knative/serving/releases/download/v0.13.0/serving-default-domain.yaml
 ```
**Caveat**: This will only work if the cluster LoadBalancer service exposes an IPv4 
address, so it will not work with IPv6 clusters, AWS, or local setups like Minikube. 
For these, we should setup a Real DNS, see docs for more.

- Monitor the state of Knative Serving components untill they all become Running or Completed
```bash
kubectl get pods --namespace knative-serving
```
