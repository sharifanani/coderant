#!/usr/bin/env bash
set -e
podman build -t coderant.dev/example_server:latest -t "$IMAGE" "$BUILD_CONTEXT"
if [ "$PUSH_IMAGE" = true ]; then
  podman push "$IMAGE"
fi
