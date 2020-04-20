### Setup GCP & create K8s Cluster

- Rescale to zero the other clusters (to avoid useless resource usage). 
With this command the `default-pool` node-pool is scaled
```bash
$ gcloud container clusters resize $CLUSTER_NAME --num-nodes=0
```
Or create a node pool which dynamically scales to zero
```bash
$ gcloud container node-pools create ${CLUSTER_NAME}-pool \
    --cluster ${CLUSTER_NAME} \
    --enable-autoscaling --min-nodes 0 --max-nodes 10 \
    --zone ${INSTANCE_ZONE}
```
And of course you still can force the scaling to zero
```bash
$ gcloud container clusters resize ${CLUSTER_NAME} \
    --size=0 [--node-pool=${CLUSTER_NAME}-pool]
```

- Install gcloud and setup account
```bash
$ gcloud config configurations create fraudio
$ gcloud config configurations list
$ gcloud init
$ gcloud activate fraudio
```

- Create a VPC network for the project. We choose regional routing because we operate only on this region for now.
```bash
$ gcloud compute networks create infinicache-network \
--subnet-mode=custom \
--bgp-routing-mode=regional 
```

- Create kubernetes VPC-native cluster and subnet simultaneously
```bash
$ gcloud container clusters create infinicache-cluster --cluster-version=1.15.9-gke.24 \
--enable-stackdriver-kubernetes --image-type=COS --machine-type=g1-small \
--disk-type=pd-standard --disk-size=50 --enable-ip-alias \
--cluster-ipv4-cidr=10.0.0.0/14 --services-ipv4-cidr=10.4.0.0/19 \
--create-subnetwork name=infinicache-subnetwork,range=10.5.0.0/20
```
