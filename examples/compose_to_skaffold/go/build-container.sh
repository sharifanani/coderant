#!/usr/bin/env bash

podman build -t coderant.dev/example_server:latest -t localhost:32000/example_server:latest .
podman push localhost:32000/example_server:latest
