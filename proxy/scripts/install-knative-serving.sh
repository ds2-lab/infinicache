#!/bin/bash

# Install the Custom Resource Definitions (aka CRDs):
kubectl apply --filename https://github.com/knative/serving/releases/download/v0.13.0/serving-crds.yaml

# Install the core components of Serving
kubectl apply --filename https://github.com/knative/serving/releases/download/v0.13.0/serving-core.yaml

# Install the Networking Layer - we choose Ambassador this time, because it seems easier to install
# Create a namespace to install Ambassador in:
kubectl create namespace ambassador
    
# Install Ambassador
kubectl apply --namespace ambassador \
  --filename https://getambassador.io/yaml/ambassador/ambassador-rbac.yaml \
  --filename https://getambassador.io/yaml/ambassador/ambassador-service.yaml
    
# Give Ambassador the required permissions:
kubectl patch clusterrolebinding ambassador -p '{"subjects":[{"kind": "ServiceAccount", "name": "ambassador", "namespace": "ambassador"}]}'
    
# Enable Knative support in Ambassador    
kubectl set env --namespace ambassador  deployments/ambassador AMBASSADOR_KNATIVE_SUPPORT=true  
    
#Configure Knative Serving to use Ambassador by default   
kubectl patch configmap/config-network \
  --namespace knative-serving \
  --type merge \
  --patch '{"data":{"ingress.class":"ambassador.ingress.networking.knative.dev"}}'
    
# Print External IP or CNAME 
kubectl --namespace ambassador get service ambassador

# Configure DNS: We ship a simple Kubernetes Job called “default domain” 
# that will (see caveats) configure Knative Serving to use xip.io as the 
# default DNS suffix
kubectl apply --filename https://github.com/knative/serving/releases/download/v0.13.0/serving-default-domain.yaml
kubectl get pods --namespace knative-serving
