### Docker

# delete images by pattern
docker images -a | grep "pattern" | awk '{print $3}' | xargs docker rmi

# Build the container on your local machine
docker build -t {username}/helloworld-go .

# Push the container to docker registry
docker push {username}/helloworld-go


### Kubernetes 

# apply config
kubectl apply --filename sample-app.yaml

# list all namespaces
kubectl get ns

# check namespace (ns) labels
kubectl get ns {namespace_name} --show-labels

# list all deployments
kubectl get deployments

# list all services
kubectl get svc

# list all triggers
kubectl get triggers

# also specifying namespace and name
kubectl --namespace {namespace} get svc {service}

# deploy a curl pod and ssh to it
kubectl --namespace {namespace} run curl --image=radial/busyboxplus:curl -it

# show logs of app
kubectl --namespace {namespace} logs -l app=helloworld-go --tail=50

# deploy a curl pod in a specific namespace and ssh to it
kubectl --namespace {namespace} run curl --image=radial/busyboxplus:curl -it