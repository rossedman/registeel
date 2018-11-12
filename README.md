# Registeel

> Tempered by pressure underground over tens of thousands of years, its body cannot be scratched.

This is a simple app that demonstrates how to build a simple Kubernetes controller. It sends metadata about pods to a mock api service which a frontend then queries and displays information from. 

- */api* - this is a simple [json-server](https://github.com/typicode/json-server) that is meant as a mock data store that controllers can be tested against
- */config* - holds the kubernetes deployment as a well as a test nginx deployment for showing how the controller updates 
- */vendor* - holds `dep` dependencies for controller
- */web* - simple vue.js frontend that contacts the api backend 

## Running Locally

This will start a kubernetes cluster and activate a registry, once your docker client is pointing at the environment running on minikube, running `make docker` will build the images on minikube so they can be deployed without being pulled from DockerHub

```
minikube start
minikube addons enable ingress
minikube addons enable registry
eval $(minikube docker-env)
make docker
kubectl apply -f config/deploy
```
