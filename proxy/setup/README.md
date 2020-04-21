# Setup

These are the steps to set Knative and its dependencies.

## Configuration

Edit [config](config) file for your setup.

## Create a GKE cluster

Create a GKE cluster *without* Istio add-on. We do this because the Istio version of the add-on usually lags behind what Knative expects:

```shell
./create-gke-cluster
```

## Install Istio

Install Istio:

```shell
./install-istio
```

## Install Knative Serving

```shell
./install-serving
```

## Install Knative Eventing

```shell
./install-eventing
```

## (Optional) Install observability features

Install observability features to enable logging, metrics, and request tracing in Serving and Eventing components:

```shell
./install-monitoring
```

## Install Knative with GCP

If you intend to read GCP Pub/Sub messages, go through these steps.

Install Knative with GCP:

```bash
./install-knative-gcp
```

Configure a Pub/Sub enabled Service Account:

```bash
./install-pubsub-serviceaccount
```

Create a Pub/Sub topic where messages will be sent:

```bash
gcloud pubsub topics create testing
```

-------