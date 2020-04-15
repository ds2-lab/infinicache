#!/bin/bash

PREFIX="infinicache-node-"
YAML_FILE="node-knService.yaml"

for ((i = 0; i <= $1; i++)); do
  YAML=$(yq w node-knService.yaml "metadata.name" $PREFIX$i)
  echo "$YAML" > "$YAML_FILE"
  kubectl delete -f node-knService.yaml
done