### Gcloud

- Scale cluster
```bash
gcloud container clusters resize $CLUSTER_NAME --num-nodes=0
``` 

- List cluster nodes (VMachines instances)
```bash
gcloud container node-pools list --cluster infinicache-cluster
```

- list accounts
```bash
gcloud auth list
```

-set active account
```bash
gcloud ocnfig set account ACCOUNT
```

-list projects
```bash
gcloud projects list
```

-set active project
```bash
gcloud config set project PROJECT
```

### Docker

* delete images by pattern
```
docker images -a | grep "pattern" | awk '{print $3}' | xargs docker rmi
```

* Build the container on your local machine
```
docker build -t {username}/helloworld-go .
```

* Push the container to docker registry
```
docker push {username}/helloworld-go
```


### Kubernetes 

* apply config
```
kubectl apply --filename sample-app.yaml
```

* list all namespaces
```
kubectl get ns
```

* check namespace (ns) labels
```
kubectl get ns {namespace_name} --show-labels
```

* list all deployments
```
kubectl get deployments
```

* list all services
```
kubectl get svc
```

* list all triggers
```
kubectl get triggers
```

* also specifying namespace and name
```
kubectl --namespace {namespace} get svc {service}
```

* deploy a curl pod and ssh to it
```
kubectl --namespace {namespace} run curl --image=radial/busyboxplus:curl -it
```

* show logs of app
```
kubectl --namespace {namespace} logs -l app=helloworld-go --tail=50
```

* deploy a curl pod in a specific namespace and ssh to it
```
kubectl --namespace {namespace} run curl --image=radial/busyboxplus:curl -it
```

* delete pods by regexp 
```
kubectl get pods --no-headers=true | awk '/proxy/ {print $1}' | xargs kubectl delete pods
```


### Other

* SSH to a remote machine
``` 
sudo ssh -i ~/.ssh/cred.pem ubuntu@35.0.0.0*____*
```

* transfer files to instance
```
scp -i ~/.ssh/FraudioAWS.pem /home/nepotu/Desktop/fraudio/projects/infinicache-master/install_go.md ubuntu@ec2-3-14-65-17.us-east-2.compute.amazonaws.com:
```

* transfer from instance to local
```
scp -i /path/my-key-pair.pem ec2-user@ec2-198-51-100-1.compute-1.amazonaws.com:~/SampleFile.txt ~/SampleFile2.txt
```

- watch log files in real time
```shell script
tail -f log
```
