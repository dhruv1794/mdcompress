# Kubernetes Fixture

Controllers reconcile desired state into cluster state.

## Concepts

A controller watches resources and writes updates through the API server.

## Manifests

```yaml
apiVersion: v1
kind: ConfigMap
metadata:
  name: example
```
