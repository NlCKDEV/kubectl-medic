# kubectl-medic

A `kubectl` plugin that helps diagnose and fix common Kubernetes issues.

## Install (dev)

```bash
git clone git@github.com:your-username/kubectl-medic.git
cd kubectl-medic
go build -o kubectl-medic ./cmd/kubectl-medic
sudo mv kubectl-medic /usr/local/bin/
