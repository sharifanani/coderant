#!/usr/bin/env bash

docker build -t coderant.dev/example_server:latest -t localhost:32000/example_server:latest .
docker push localhost:32000/example_server:latest
