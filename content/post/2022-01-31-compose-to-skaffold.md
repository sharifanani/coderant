---
title: Moving a docker-compose workflow to Kubernetes with Skaffold
author:
  - Sharif Anani
date: '2022-01-31'
categories:
  - Containers
tags:
  - Docker
  - Kubernetes
  - Podman
  - Containers
  - Skaffold
  - K8s
toc: true
---

## Introduction and motivation
In this post, we'll go through creating a small stub server using Go, and we'll deploy it in a development environment
using `docker-compose`. After that, we'll see (and do) what it takes to move this workflow to use kubernetes to manage
the containers instead. 

This post is partially motivated by docker's recent [changes to their licensing model](https://www.docker.com/blog/updating-product-subscriptions/)
and partially motivated by just wanting to learn more about kubernetes and help close the gap between workloads in production and workloads in development.

## Prerequisites
* Have Go installed. Head to https://go.dev/doc/install if you don't.

## Project setup

Start by creating an empty directory for your project, I like to keep my projects under `~/code/`, so I'll run
```shell
mkdir -p ~/code/compose_to_skaffold && cd ~/code/compose_to_skaffold
```
Once inside the directory, create the directory where your server code will live
```shell
mkdir -p go/src && cd go/src
```

Once inside `go/src`, create a new go module for your server with
```shell
go mod init coderant.dev/tiny_server
```
I don't think creating the go module is strictly necessary, but why not? It makes it a lot easier to manage dependencies (and be a dependency if needed)

Next, create a `main.go` file with the following contents for the server
```go
// compose_to_skaffold/go/src/main.go
package main

import (
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
)

func main() {
	port := flag.Int("port", 8080, "Port number to use")
	ip := flag.String("ip", "127.0.0.1", "IP of interface to listen on")
	flag.Parse()
	// simple handler on the default mux
	http.HandleFunc("/hello", func(writer http.ResponseWriter, request *http.Request) {
		log.Default().Println("received request")
		_, err := writer.Write([]byte("Hello there!"))
		if err != nil {
			writer.WriteHeader(http.StatusInternalServerError)
			_, _ = writer.Write([]byte(err.Error()))
		}
	})
	listenAddr := fmt.Sprintf("%s:%d", *ip, *port)
	log.Default().Printf("listening on: %s \n", listenAddr)
	err := http.ListenAndServe(listenAddr, nil)
	if err != http.ErrServerClosed {
		log.Default().Fatalf("encountered error: %v", err)
	}
	os.Exit(0)
}
```
Test things out by running `go run main.go -port 9090 -ip 0.0.0.0`
```shell
$ go run main.go -port 9090 -ip 0.0.0.0
2022/01/31 22:51:23 listening on: 0.0.0.0:9090
```

In another terminal, run `curl localhost:9090/hello` to test your server. Your terminal should look something like this:
```shell
> curl localhost:9090/hello
Hello there!⏎
```
Awesome! We can move on with our lives noe

## Package your server in a container
To deploy the server using a container, we'll need to [install `docker`](https://docs.docker.com/engine/install/ubuntu/).
Head over to https://docs.docker.com/engine/install/ubuntu/ and follow their instructions.

Once installed, create the file `Dockerfile` with the following contents:
```dockerfile
FROM docker.io/golang:stretch
COPY ./src /app
WORKDIR /app/
CMD go run main.go -ip 0.0.0.0 -port 8080
```

We can build the container with `docker build`, but we're not amateurs here, we're going to write a script that runs it for us.

Create the file `go/build-container.sh` with the contents
```shell
#!/usr/bin/env bash

docker build -t coderant.dev/example_server:latest ./go
````

We'll expand on this file a little later. For now, don't forget to make it executable
```shell
chmod +x build-container.sh
```

To build the container:
```shell
$ pwd
/path/to/your/project/compose_to_skaffold
$ ./build-container.sh
Sending build context to Docker daemon  7.168kB
Step 1/4 : FROM docker.io/golang:stretch
stretch: Pulling from library/golang
a834d7c95167: Pull complete 
57b3fa6f1b88: Pull complete 
778df3ecaa0f: Pull complete 
d353c340774e: Pull complete 
b9b15e1d90c2: Pull complete 
812024fc77bd: Pull complete 
bf9c5d10aa4d: Pull complete 
Digest: sha256:83a2dd5267864cb4c15a78f8722a6bc0d401d4baec358695f5dc5dd1067d15d1
Status: Downloaded newer image for golang:stretch
 ---> 9e153dc8839b
Step 2/4 : COPY ./src /app
 ---> f9a609eb26d7
Step 3/4 : WORKDIR /app/
 ---> Running in 157139c2dd6b
Removing intermediate container 157139c2dd6b
 ---> b21729ab364c
Step 4/4 : CMD go run main.go -ip 0.0.0.0 -port 8080
 ---> Running in e3932bb13fc3
Removing intermediate container e3932bb13fc3
 ---> 11c2bf0d0189
Successfully built 11c2bf0d0189
Successfully tagged coderant.dev/example_server:latest
```
## Deploy with `docker-compose`

If you're reading this, there's a good chance you're already pretty familiar with `docker-compose` and already using it.
If that's not the case, `docker-compose` is a convenient command that lets you use a [`docker-compose.yml` file](https://docs.docker.com/compose/compose-file/compose-file-v2/)
to specify a *composition* of containers. In the root directory of your project, create a file called `docker-compose.yml`
with the following contents:
```yaml
version: "2"
services:
  backend:
    container_name: example_backend
    image: "coderant.dev/example_server:latest"
    ports:
      - "9090:8080"
```

From that same directory, you can run
```shell
$ docker-compose up
Creating example_backend ... done
Attaching to example_backend
example_backend | 2022/02/02 00:00:38 listening on: 0.0.0.0:8080
```

Congratulations! Your server is now running, and it's running from a container "orchestrated" by docker-compose.
Pat yourself on the back pls.

It is worth noting than in an actual development environment, we would want the server to rebuild and restart on code changes.
We'll handle that in a later part of this post when we're all set up with k8s and skaffold.

## Enter kubernetes
Alright, we're here now.

To start, we need a kubernetes distribution that runs locally. I chose [microk8s](https://microk8s.io/). Seemed like a
the logical option for targeting something that's not too complicated, but also not completely stripped down.

Head over to https://microk8s.io/ if you want to get the installation instructions from the source, but  in short, you'll be running
```shell
$ sudo snap install microk8s --classic
```

Once done, you should enable some basic services with
```shell
microk8s enable dns registry istio traefik
```
The `registry` addon will allow us to push containers to a registry that's accessible by `microk8s`.
Alternatively, we could use the method suggested in the [`microk8s` docs](https://microk8s.io/docs/registry-images),
but I like pushing more than save/load.

The `traefik` addon will create what's called an [Ingress Controller](https://kubernetes.io/docs/concepts/services-networking/ingress-controllers/).
It's essentially a proxy that allows data coming in to the cluster to reach its destination [Service](https://kubernetes.io/docs/concepts/services-networking/service/),
which exposes ports on [Deployments](https://kubernetes.io/docs/concepts/workloads/controllers/deployment/).

Finally, a useful alias to have would be `microk8s kubectl` -> `kubectl`. If you're using `bash` as your shell, add
```shell
alias kubectl="microk8s kubectl"
```
to the end of your `~/.bashrc` file and restart your shell or `source ~/.bashrc`.

You should now be able to run
```shell
$ kubectl get nodes
NAME      STATUS   ROLES    AGE     VERSION
kubuntu   Ready    <none>   3d19h   v1.23.3-2+d441060727c463
```

### Monitor and inspect your cluster with k9s

This is totally optional, but I prefer it to the stock dashboard. Head over to https://k9scli.io/topics/install/
and follow their instructions (I went the binary route).

In order for k9s to connect to your cluster, it'll need a good kube config file. You can provide that by running
```shell
$ mkdir ~/.kube
$ microk8s config dump > ~/.kube/config
```

By default, `microk8s` expects you to use `microk8s kubectl` and not really need that config file elsewhere.
We'll use `k9s` later, but for now, we'll let it simmer. you can run `k9s` to take a look around if you want.

### Adjust the build script to use the microk8s registry
For microk8s to find the container, it should be pushed to its registry.
This is a simple adjustment in `build-container.sh`:
```shell
#!/usr/bin/env bash
set -e
docker build -t coderant.dev/example_server:latest -t localhost:32000/example_server:latest ./go
docker push localhost:32000/example_server:latest
```

Once modified, go ahead and run the script with `./build-container.sh`. Your local microk8s registry is now armed and ready!

## Putting together a manifest and deploying your server.
Similar to how `docker-compose.yml` files can be used to tell the docker engine to deploy a specific set of containers
with a specific set of priviliges/peripherals/devices/etc.., in kubernetes, we also use `.yml` files to compile what we'll
be referring to as a *manifest*.

Right next to your `docker-compose.yml`, create a `manifest.yml` and open it for editing. This might take a minute.

### The deployment
The first section we'll be adding to the manifest will be the [Deployment](https://kubernetes.io/docs/concepts/workloads/controllers/deployment/).
If you're in a hurry, this is the section to be added:
```yaml
---
apiVersion: apps/v1
kind: Deployment
metadata:
  name: example-deployment
spec:
  selector:
    matchLabels:
      app: example-backend
  template:
    metadata:
      labels:
        app: example-backend
    spec:
      containers:
      - name: example
        image: localhost:32000/example_server:latest
        resources:
          requests:
            memory: "32Mi"
            cpu: "100m"
          limits:
            memory: "128Mi"
            cpu: "500m"
        ports:
        - containerPort: 8080
```
The deployment is what manages running/stopping/scaling/etc of the containers through [ReplicaSets](https://kubernetes.io/docs/concepts/workloads/controllers/replicaset/).
We won't be messing with ReplicaSets directly. Why? because k8s docs say so 👍.

We can stop here and call it a day, but we wouldn't really be able to access the webapp very smoothly. 
We can, however, use k9s to poke around!

Save the current changes to `manifest.yaml` and run:
```shell
kubectl apply -f manifest.yaml
```
Your terminal should look like this:
```shell
$ k apply -f manifest.yaml
deployment.apps/example-deployment created
```
Now, run `k9s`, and then type `:deployment` and hit the enter key.
Your terminal will show something like
```shell
 Context: microk8s                                 <0> all       <ctrl-d> Del… ____  __.________        
 Cluster: microk8s-cluster                         <1> traefik   <d>      Desc|    |/ _/   __   \______ 
 User:    admin                                    <2> default   <e>      Edit|      < \____    /  ___/ 
 K9s Rev: v0.25.5 ⚡️v0.25.18                                     <?>      Help|    |  \   /    /\___ \  
 K8s Rev: v1.23.3-2+d441060727c463                               <l>      Logs|____|__ \ /____//____  > 
 CPU:     n/a                                                    <p>      Logs        \/            \/  
 MEM:     n/a                                                                                           
┌────────────────────────────────────── Deployments(default)[1] ───────────────────────────────────────┐
│ NAME↑                                 READY          UP-TO-DATE          AVAILABLE AGE               │
│ example-deployment                      1/1                   1                  1 4m43s             │
│                                                                                                      │
│                                                                                                      │
│                                                                                                      │
│                                                                                                      │
│                                                                                                      │
│                                                                                                      │
│                                                                                                      │
│                                                                                                      │
│                                                                                                      │
│                                                                                                      │
│                                                                                                      │
│                                                                                                      │
└──────────────────────────────────────────────────────────────────────────────────────────────────────┘
  <deployment>                                                                                          
```
You select your deployment (by highlighting it and pressing enter) and that'll show you an IP for the pod.
You can `curl <pod up>:8080/hello` to see your server running!
```shell
$ curl 10.1.150.168:8080/hello
Hello there!⏎
```

If you don't want to use `k9s`, you can
```shell
$ kubectl get pods -o json | grep ip
                        "ip": "10.1.150.168"
```
It's faster, but only about a quarter as cool. The choice is yours. If you have multiple pods, you'd have to do something like
```shell
$ kubectl get pods
NAME                                  READY   STATUS    RESTARTS   AGE
example-deployment-7d975b57b5-fps46   1/1     Running   0          18m
$ kubectl get pods example-deployment-7d975b57b5-fps46 -o json | grep ip
                "ip": "10.1.150.168"
```

Alriiiight, we're cooking with gas now! Clean up after yourself and run
```shell
$ kubectl delete -f manifest.yaml
```

### The service and ingress
The pod IP that we just used is automatically assigned by the cluster and could change whenever we redeploy.
To have something more repeatable, we'll use the combination of a [Service](https://kubernetes.io/docs/concepts/services-networking/service/)
and an [Ingress](https://kubernetes.io/docs/concepts/services-networking/ingress/).

With `docker-compose`, we're able to just say `ports: "hostport:containerport"` and call it a day. That's because
in `docker-compose`, the number of containers is assumed to be `1`.

In Kubernetes, a *Service* basically says
*"I'll listen on port X, and whatever I receive, I'll forward to one of the containers that match what I have defined in my spec"*
which is a good start! I'm sure there's a better analogy with `docker swarm`, but I'm not familiar with it. [Add me on LinkedIn if you want to talk about it!](https://www.linkedin.com/in/sharif-anani/)

The *Ingress* is basically your run-of-the-mill proxy config! 
It basically says *"I'll listen for traffic coming in from __outside__ of the cluster, and depending on my spec, I'll forward the traffic to the right Service"*

Without further ado, append this to your `manifest.yaml`
```yaml
---
apiVersion: v1
kind: Service
metadata:
  name: example-backend-service
spec:
  type: ClusterIP
  selector:
    app: example-backend
  ports:
  - port: 8080
    targetPort: 8080
    name: http
---
apiVersion: networking.k8s.io/v1
kind: Ingress
metadata:
  name: example-backend-ingress
spec:
  rules:
    - host: example-backend.local
      http:
        paths:
          - backend:
              service:
                name: example-backend-service
                port:
                  name: http
            pathType: Prefix
            path: /
```
The service here is a [ClusterIP](https://kubernetes.io/docs/concepts/services-networking/service/#publishing-services-service-types)
service which basically creates a listener that listens on an assigned IP within the cluster and forwards traffic to the app specified in the selector.

The Ingress will be noticed by the `traefik` ingress controller that we enabled earlier, and it will cause traefik to forward
traffic coming in on `example-backend.local` to the service `example-backend-service`, which in turn forwards the traffic to
one of the pods with the label `app: example-backend`.

Remember, `traefik` is actually listening on `loopback:8080`, so you'll want to go to `example-backend.local:8080/hello`
to see your server in action.

Save your `manifest.yaml` and re-run:
```shell
$ kubectl apply -f manifest.yaml
```

Your terminal should show something like
```shell
deployment.apps/example-deployment created
service/example-backend-service created
ingress.networking.k8s.io/example-backend-ingress created
```
Before you can actually access the server, add `127.0.0.1 example-backend.local` to your `/etc/hosts` file.

```shell
$ curl example-backend.local:8080/hello
Hello there!⏎
```

Awesome! We've successfully moved to using k8s now!

Again, clean up after yourself with 
```shell
$ kubectl delete -f manifest.yaml
```
This whole `kubectl apply` then `kubectl delete` business is getting old, isn't it?

Before moving forward, let's clean up the microk8s registry by running the folllowing command:

```shell
$ microk8s ctr image rm $(microk8s ctr image ls name~="example" -q)
```

## Live refresh and dev workflow with Skaffold
Start by [installing skaffold](https://skaffold.dev/docs/install/#standalone-binary).
When done, create `skaffold.yaml` right next to your `manifest.yaml`
```yaml
apiVersion: skaffold/v2beta26
kind: Config
metadata:
  name: compose-to-skaffold
build:
  artifacts:
    - image: localhost:32000/example_server
      context: go
      custom:
        buildCommand: ../build-container.sh
deploy:
  kubectl:
    manifests:
      - manifest.yaml
```

Notice a couple of things here:
* we're not using a tag with the docker image
* the `build-container.sh` script is specified relative to the `context` specified

For this to work, we need our build script to honor [contract that skaffold expects to be honored](https://skaffold.dev/docs/pipeline-stages/builders/custom/#contract-between-skaffold-and-custom-build-script)

*imagine a non-copyright-protected image of a gangster pounding his chest and nodding here*

^ I don't know much about copyright law. Anyway, I digress.

The new and improved `build-container.sh` should look like the following:
```shell
#!/usr/bin/env bash
set -ex
docker build -t coderant.dev/example_server:latest -t "$IMAGE" "$BUILD_CONTEXT"
if [ "$PUSH_IMAGE" = true ]; then
  docker push "$IMAGE"
fi
```

When you're done, go ahead and run `skaffold dev` to start things up!

```shell
$ skaffold dev
Listing files to watch...
 - localhost:32000/example_server
Generating tags...
 - localhost:32000/example_server -> localhost:32000/example_server:bb61adf-dirty
Checking cache...
 - localhost:32000/example_server: Not found. Building
Starting build...
Building [localhost:32000/example_server]...
+ docker build -t coderant.dev/example_server:latest -t localhost:32000/example_server:bb61adf-dirty <path-to-context>
Sending build context to Docker daemon  6.144kB
Step 1/4 : FROM docker.io/golang:stretch
 ---> 9e153dc8839b
Step 2/4 : COPY ./src /app
 ---> Using cache
 ---> 369aa40461c7
Step 3/4 : WORKDIR /app/
 ---> Using cache
 ---> 142a6f7626a9
Step 4/4 : CMD go run main.go -ip 0.0.0.0 -port 8080
 ---> Using cache
 ---> 6d23ab99103e
Successfully built 6d23ab99103e
Successfully tagged coderant.dev/example_server:latest
Successfully tagged localhost:32000/example_server:bb61adf-dirty
+ '[' true = true ']'
+ docker push localhost:32000/example_server:bb61adf-dirty
The push refers to repository [localhost:32000/example_server]
907483c1f563: Preparing
c7bbd574971a: Preparing
eefed195ceec: Preparing
58f0ebb0cceb: Preparing
76ce09dad18e: Preparing
9aee2e50701e: Preparing
678c62bc4ece: Preparing
d05b8af4c7ce: Preparing
9aee2e50701e: Waiting
678c62bc4ece: Waiting
d05b8af4c7ce: Waiting
907483c1f563: Layer already exists
58f0ebb0cceb: Layer already exists
eefed195ceec: Layer already exists
678c62bc4ece: Layer already exists
c7bbd574971a: Layer already exists
d05b8af4c7ce: Layer already exists
9aee2e50701e: Layer already exists
76ce09dad18e: Layer already exists
bb61adf-dirty: digest: sha256:9370b9ae1f92f79ceb4044b6756f8e37a6ca8fd0062c188091a1e0ecf06003b7 size: 2003
Tags used in deployment:
 - localhost:32000/example_server -> localhost:32000/example_server:bb61adf-dirty@sha256:9370b9ae1f92f79ceb4044b6756f8e37a6ca8fd0062c188091a1e0ecf06003b7
Starting deploy...
 - deployment.apps/example-deployment created
 - service/example-backend-service created
 - ingress.networking.k8s.io/example-backend-ingress created
Waiting for deployments to stabilize...
 - deployment/example-deployment is ready.
Deployments stabilized in 2.111 seconds
Press Ctrl+C to exit
Watching for changes...
[example] 2022/02/03 03:16:26 listening on: 0.0.0.0:8080
```

In another terminal, do your `curl` thing!
```shell
$ curl example-backend.local:8080/hello
Hello there!⏎
```

Now, change something your `main.go`, and you'll notice that `skaffold` build and redeploys for you!

## Getting rid of docker once and for all

The final thing we should do now is get rid of docker!

To do that, head over to https://podman.io/getting-started/installation and follow their instructions.
When done, update `build-container.sh` by replacing `docker` with `podman` everywhere:

```shell
#!/usr/bin/env bash
set -e
podman build -t coderant.dev/example_server:latest -t "$IMAGE" "$BUILD_CONTEXT"
if [ "$PUSH_IMAGE" = true ]; then
  podman push "$IMAGE"
fi
```

It might be useful to change something in `main.go` to make sure that the hashes get mangled, and then re-run `skaffold dev`

## Some obvious issues & closing remarks
Obviously, re-building the container each time is fairly impractical (unless the `Dockerfiles` are very well crafted).
To address that, we'd use `skaffold`'s `sync:` directive, and some hooks that would run in the container post-sync 👍.

I'll make sure to post something here soon to show how we can do just that!

Thanks for reading!