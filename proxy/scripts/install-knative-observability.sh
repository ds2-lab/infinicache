#!/bin/bash

# create knative-monitoring namespace & install the core
kubectl apply --filename https://github.com/knative/serving/releases/download/v0.13.0/monitoring-core.yaml

# install prometeus  & graphana for metrics
kubectl apply --filename https://github.com/knative/serving/releases/download/v0.13.0/monitoring-metrics-prometheus.yaml

# install the ELK stack (Elasticsearch, Logstash and Kibana) for logs:
kubectl apply --filename https://github.com/knative/serving/releases/download/v0.13.0/monitoring-logs-elasticsearch.yaml

# enable request metrics by adding `metrics.request-metrics-backend-destination: prometheus` to data field
# kubectl edit cm -n knative-serving config-observability

# or just edit logsk8sConfig.yaml file and then apply it
kubectl apply -f ../logging-cm.yaml

# install knative core observability plugin
kubectl apply --filename https://github.com/knative/serving/releases/download/v0.13.0/monitoring-core.yaml

# install prometeus and graphana for metrics
kubectl apply --filename https://github.com/knative/serving/releases/download/v0.13.0/monitoring-metrics-prometheus.yaml

# install the ELK stack (Elasticsearch, Logstash and Kibana) for logs
kubectl apply --filename https://github.com/knative/serving/releases/download/v0.13.0/monitoring-logs-elasticsearch.yaml

# check monitoring pods
kubectl get pods --namespace knative-monitoring --watch

# enable request logs, copy logging.request-log-template from data._example field to data field in the ConfigMap you are editing
kubectl edit cm -n knative-serving config-observability

## setup stackdriver plugin

# clone the Knative Serving repository (it contains k8s manifests for installation)
git clone -b v0.13.0 https://github.com/knative/serving knative-serving
cd knative-serving

# configure the DaemonSet for stdout/stderr logs
# (modify the manifest with own conf if needed)
kubectl apply -f 100-fluentd-configmap.yaml

# setup desired fluentd image and plugins of fluentd DaemonSet according with
# https://knative.dev/docs/serving/fluentd-requirements/#requirements
# and then apply config
kubectl apply -f 200-fluentd.yaml

# configure the DaemonSet for log files in /var/log by setting
# logging.enable-var-log-collection to true in config-observability (logging-cm.yaml.yaml)
kubectl apply -f logging-cm.yaml

# deploy the configuration for enabling /var/log collection
kubectl apply --filename config/config-observability.yaml

# Deploy the DaemonSet to make configuration for DaemonSet take effect
  kubectl apply --recursive --filename config/monitoring/100-namespace.yaml \
      --filename config/monitoring/logging/stackdriver/100-fluentd-configmap.yaml

# uninstall plugin
kubectl delete --recursive --filename config/monitoring/logging/stackdriver/100-fluentd-configmap.yaml

# install knative stackdriver components
kubectl apply --recursive --filename config/monitoring/100-namespace.yaml --filename config/monitoring/logging/stackdriver