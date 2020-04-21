#!/bin/bash

PREFIX="infinicache-node-"
YAML_FILE="node-knService.yaml"

for ((i = 0; i <= $1; i++)); do
  YAML=$(yq w node-knService.yaml "metadata.name" $PREFIX$i)
  echo "$YAML" > "$YAML_FILE"
  kubectl apply -f node-knService.yaml
  kubectl label kservice $PREFIX$i serving.knative.dev/visibility=cluster-local
done
kubectl get ksvc -o=jsonpath={.items[*].status.address.url}

