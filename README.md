# KubeAid Cluster Bootstrap Script

This script 

> It currently supports Linux only. In order to bring support for MacOS, we need to make the KubeAid kube-prometheus build script compatible with MacOS.

## PREREQUISITES

You need these CLI tools - `jsonnet`, `gojsontoyaml` and `kubeseal`. If you don't have these installed, then the script will do it for you.

You manually need to create a `local Kubernetes cluster` with `ArgoCD` and `Sealed Secrets` installed. You can use these commands :
```sh
# Create a local K3S cluster.
k3d cluster create \
	--image rancher/k3s:v1.27.15-k3s2 \
	--servers 1 --agents 2

# Install ArgoCD.
helm repo add argo https://argoproj.github.io/argo-helm
helm install argocd argo/argo-cd -n argocd --create-namespace --wait

# Install Sealed Secrets.
helm repo add sealed-secrets https://bitnami-labs.github.io/sealed-secrets
helm install sealed-secrets sealed-secrets/sealed-secrets -n kube-system --wait
```

## TODOS

- [] Help the user, update the cluster.
