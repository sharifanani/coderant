---
title: FileSync to speed up development workflows with Skaffold
author:
  - Sharif Anani
date: '2022-02-12'
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

## Before reading this post
Before reading this post, check out [the previous post](/post/2022-01-31-compose-to-skaffold)
to see how we got here.

## Adding FileSync
[Skaffold](https://skaffold.dev) watches for changes and rebuilds the containers by default. However, completely
rebuilding the container can quickly become very impractical.

To get around this, we can use the [FileSync](https://skaffold.dev/docs/pipeline-stages/filesync/) to tell Skaffold to
copy changes to an already-built container instead of rebuilding the whole thing.
For our project from [the previous post](/post/2022-01-31-compose-to-skaffold), this will be fairly simple:
adjust your `skaffold.yaml` to look like the following:

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
    sync:
      manual:
        - src: 'src/*'
          dest: /app/
          strip: 'src/'
deploy:
  kubectl:
    manifests:
      - manifest.yaml
```

After updating, go ahead and run `skaffold dev` from the project root, and then "Hello there!" in your `main.go` to
"Hello there! I've changed!".

The expected output should be something like the following:
```shell
$ skaffold dev
Listing files to watch...
 - localhost:32000/example_server
Generating tags...
 - localhost:32000/example_server -> localhost:32000/example_server:d31182c-dirty
Checking cache...
 - localhost:32000/example_server: Found. Tagging
Tags used in deployment:
 - localhost:32000/example_server -> localhost:32000/example_server:d31182c-dirty@sha256:4334fcc3418d4697cdcae28b2538339055d1413b3ce6b9f94071f7319df17efa
Starting deploy...
 - deployment.apps/example-deployment created
 - service/example-backend-service created
 - ingress.networking.k8s.io/example-backend-ingress created
Waiting for deployments to stabilize...
 - deployment/example-deployment is ready.
Deployments stabilized in 2.07 seconds
Press Ctrl+C to exit
Watching for changes...
[example] 2022/02/13 00:37:07 listening on: 0.0.0.0:8080 
[example] 2022/02/13 00:37:12 received request
##### this is what happens when a file is changed
Syncing 1 files for localhost:32000/example_server:d31182c-dirty@sha256:4334fcc3418d4697cdcae28b2538339055d1413b3ce6b9f94071f7319df17efa
Watching for changes...
[example] 2022/02/13 00:37:21 received request
```

However, if you check the output, you still get
```shell
$ curl example-backend.local:8080/hello
Hello there!⏎                                   
```

Why is that? It's because Skaffold is no longer doing a full rebuild, it's replacing the files in the already-built containers.
However, because we're not restarting the container's `go run` command, we can't see the changes happening

## Managing the server process

When we created our container initially, our `Dockerfile` ended with
```dockerfile
CMD go run main.go -ip 0.0.0.0 -port 8080
```
In general, when a conntainers `CMD` (or command) exits, the container also exits. To facilitate a good workflow,
we'll want our container command to be a long-running process (think daemon or service) that runs in the foreground and
managed our web server.

There are numerous solutions for this, and a tried-and-true one is [`supervisor`](http://supervisord.org/).
To add `supervisord` to the container, we'll want to do a few things:

1. Add `supervisor` to the build by updating the Dockerfile
2. Create a `supervisor` config file to specify our job
3. Change our `CMD` to run supervisor instead of `go run` directly.
4. Add post-sync hooks to recompile the server and restart the process

### Adding supervisor

Adding `supervisor` to the build is pretty simple; update the `Dockerfile` to include the `apt install` command:
```dockerfile
FROM docker.io/golang:stretch
RUN apt-get update && apt-get install -y supervisor
COPY ./src /app
WORKDIR /app/
CMD go run main.go -ip 0.0.0.0 -port 8080
```

### Create a supervisor config file
The [`supervisord` documentation](http://supervisord.org/configuration.html) is actually pretty good at laying out
the available options and the minimum set of options needed to specify a job.
Our job will be simple: run an executable with a certain command line, and dump all logs directly to `stdout` 👍

Add the following to the file `go/supervisord.conf`, right next to your `Dockerfile`.
I chose this location to keep it included in the context for easy copying.
```toml
[program:example_backend]
command=./example_server -port 8080 -ip 0.0.0.0
redirect_stderr=true
stdout_logfile=/dev/stdout
stdout_logfile_maxbytes=0
numprocs=1
numprocs_start=0
```
Notice the `command` directive. It'll run the `example-server` command in the working directory with the `-ip` and `-port`
options.

### Running supervisor instead of the `go run`

For the `command` specified in `supervisord.conf` to work, we're going to change how we build and run the server.
We'll want to build the server during the container build and place the executable in the same working directory expected by
supervisor, and we'll also want to place the `supervisord.conf` file to one of the directories that `supervisord` expects.

The updated `Dockerfile` will look like the following:
```dockerfile
FROM docker.io/golang:stretch
RUN apt-get update && apt-get install -y supervisor
COPY ./src /app
WORKDIR /app/
RUN go build -o ./example_server main.go
COPY supervisord.conf /etc/supervisor/conf.d/example_backend.conf
CMD supervisord -n -c /etc/supervisor/supervisord.conf
```
Notice that we now `RUN go build` instead of `CMD go run`. This compiles the server ahead of time for the first run.
After that, we copy over the `supervisord.conf` file to `etc/supervisor/conf.d/example_backend.conf`. This directory
is one of the default directories where supervisor will look for more configurations, and the whole directory is included
as per the default configuration placed in `/etc/supervisor/supervisord.conf`. You can take a look at the default config file yourself:
```shell
$ podman run -it --rm localhost:32000/example_server:d31182c-dirty cat /etc/supervisor/supervisord.conf
```
```toml
; supervisor config file

[unix_http_server]
file=/var/run/supervisor.sock   ; (the path to the socket file)
chmod=0700                       ; sockef file mode (default 0700)

[supervisord]
logfile=/var/log/supervisor/supervisord.log ; (main log file;default $CWD/supervisord.log)
pidfile=/var/run/supervisord.pid ; (supervisord pidfile;default supervisord.pid)
childlogdir=/var/log/supervisor            ; ('AUTO' child log dir, default $TEMP)

; the below section must remain in the config file for RPC
; (supervisorctl/web interface) to work, additional interfaces may be
; added by defining them in separate rpcinterface: sections
[rpcinterface:supervisor]
supervisor.rpcinterface_factory = supervisor.rpcinterface:make_main_rpcinterface

[supervisorctl]
serverurl=unix:///var/run/supervisor.sock ; use a unix:// URL  for a unix socket

; The [include] section can just contain the "files" setting.  This
; setting can list multiple files (separated by whitespace or
; newlines).  It can also contain wildcards.  The filenames are
; interpreted as relative to this file.  Included files *cannot*
; include files themselves.

[include]
files = /etc/supervisor/conf.d/*.conf
```

Finally, we run `supervisor` with the line
```dockerfile
CMD supervisord -n -c /etc/supervisor/supervisord.conf
```
The `-n` option specifies that we want to run in the "no daemon" mode, which keeps supervisor running as a foreground
process preventing the container from exiting, unless supervisor itself exits 
(which is less likely to happen, even if your code errors out).

The `-c` options lets us specify the config to use.

### Add post-sync hooks

Skaffold's pretty good about offering to run scripts for you at different points in the lifecycle.
The [documentation](https://skaffold.dev/docs/pipeline-stages/lifecycle-hooks/) goes into details about the available hooks,
but we'll be using the [`after-sync`](https://skaffold.dev/docs/pipeline-stages/lifecycle-hooks/#before-sync-and-after-sync) hooks
to do what we need.

Add a `hooks` key to your `sync` key in `skaffold.yaml`:
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
    sync:
      manual:
        - src: 'src/*'
          dest: /app/
          strip: 'src/'
      hooks:
        after:
          - container:
              command: ["go", "build", "-o", "./example_server", "main.go"]
          - container:
              command: ["supervisorctl", "restart", "all"]
deploy:
  kubectl:
    manifests:
    - manifest.yaml
```

The `after` key here means that we want this to happen after the sync is done, and the value is an array of `container`
or `host` commands to run.

Our first command re-compiles the executable:
```yaml
          - container:
              command: ["go", "build", "-o", "./example_server", "main.go"]
```
And the second command restarts all supervisor programs:
```yaml
          - container:
              command: ["supervisorctl", "restart", "all"]
```

That's all, folks! with all the changes in place, re-run `skaffold dev`, and notice what happens when you change a file:
```shell
$ skaffold dev
Listing files to watch...
 - localhost:32000/example_server
Generating tags...
 - localhost:32000/example_server -> localhost:32000/example_server:d31182c
Checking cache...
 - localhost:32000/example_server: Found Remotely
Tags used in deployment:
 - localhost:32000/example_server -> localhost:32000/example_server:d31182c@sha256:b6222d0291acdc41c68fe79f91c8b8259a9a2f73906a51861cfbaad137f6de05
Starting deploy...
 - deployment.apps/example-deployment created
 - service/example-backend-service created
 - ingress.networking.k8s.io/example-backend-ingress created
Waiting for deployments to stabilize...
 - deployment/example-deployment is ready.
Deployments stabilized in 2.066 seconds
Press Ctrl+C to exit
Watching for changes...
[example] 2022-02-13 01:17:44,266 CRIT Supervisor running as root (no user in config file)
[example] 2022-02-13 01:17:44,266 INFO Included extra file "/etc/supervisor/conf.d/example_backend.conf" during parsing
[example] 2022-02-13 01:17:44,276 INFO RPC interface 'supervisor' initialized
[example] 2022-02-13 01:17:44,276 CRIT Server 'unix_http_server' running without any HTTP authentication checking
[example] 2022-02-13 01:17:44,276 INFO supervisord started with pid 9
[example] 2022-02-13 01:17:45,278 INFO spawned: 'example_backend' with pid 12
[example] 2022/02/13 01:17:45 listening on: 0.0.0.0:8080 
[example] 2022-02-13 01:17:46,283 INFO success: example_backend entered RUNNING state, process has stayed up for > than 1 seconds (startsecs)
[example] 2022/02/13 01:18:24 received request
Syncing 1 files for localhost:32000/example_server:d31182c@sha256:b6222d0291acdc41c68fe79f91c8b8259a9a2f73906a51861cfbaad137f6de05
Starting post-sync hooks for artifact "localhost:32000/example_server"...
[example] example_backend: stopped
[example] example_backend: started
Completed post-sync hooks for artifact "localhost:32000/example_server"
Watching for changes...
[example] 2022/02/13 01:18:46 received request
```

Without having to restart skaffold, `curl`ing shows that we can see the changes
```shell
$ curl example-backend.local:8080/hello
Hello there!⏎                                                                                                                                                                                                  anani@kubuntu ~/s/blog (refresh-without-build)> curl example-backend.local:8080/hello
$ curl example-backend.local:8080/hello
Hello there! I've Changed!⏎
```