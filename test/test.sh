#!/bin/sh

CKE_HOST=10.124.142.224:8512

#create cluster
curl -H "Content-Type:application/json" -X POST http://$CKE_HOST/k8s/clusters --data @create.json
