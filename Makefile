VERSION=0.2.5
BUILD=$(shell git rev-parse --short HEAD)
scheduler_image_name=reg.mg.hcbss/open/cke-scheduler
wrapper_image_name=reg.mg.hcbss/open/cke-k8s-wrapper

mkfile_path :=$(shell pwd)
export GOPATH=$(shell readlink -f $(mkfile_path)/../../)

.PHONY: all scheduler k8s k8s-proto k8s-exec k8s-wrapper kvm images scheduler-image wrapper-image base_image test help

all: scheduler k8s kvm images

scheduler: k8s-proto
	@echo "===== build cke-scheduler"
	CGO_ENABLED=0 GOOS=linux go build -ldflags "-X 'cke.BUILD=$(BUILD)' -X 'cke.VERSION=$(VERSION)' -s -w -extldflags '-static'" -o build/bin/cke-scheduler main/main.go

k8s:	k8s-exec k8s-wrapper k8s-cke-proxy
	@echo "build cke-k8s-exec and cke-k8s-wrapper"
k8s-exec: k8s-proto
	@echo "===== build cke-k8s-exec"
	CGO_ENABLED=0 GOOS=linux go build -ldflags "-X 'cke.BUILD=$(BUILD)' -X 'cke.VERSION=$(VERSION)' -s -w -extldflags '-static'" -o html/build/exec/cke-k8s-exec kubernetes/executor/main/main.go

k8s-wrapper: k8s-proto
	@echo "===== build cke-k8s-wrapper"
	CGO_ENABLED=0 GOOS=linux go build -ldflags "-X 'cke.BUILD=$(BUILD)' -X 'cke.VERSION=$(VERSION)' -s -w -extldflags '-static'" -o build/bin/cke-k8s-wrapper kubernetes/wrapper/main/main.go

k8s-proto: kubernetes/k8s_task.pb.go kubernetes/kube_node.pb.go
kubernetes/k8s_task.pb.go: kubernetes/k8s_task.proto
	protoc --gogo_out=plugins=grpc:./ --proto_path=./ -I./vendor -I/usr/local/protoc/include ./kubernetes/k8s_task.proto
kubernetes/kube_node.pb.go: kubernetes/kube_node.proto
	protoc --gogo_out=plugins=grpc:./ --proto_path=./ -I./vendor -I/usr/local/protoc/include ./kubernetes/kube_node.proto
k8s-cke-proxy:
	@echo "===== build k8s-cke-proxy"
	CGO_ENABLED=0 GOOS=linux go build -ldflags "-X 'cke.BUILD=$(BUILD)' -X 'cke.VERSION=$(VERSION)' -s -w -extldflags '-static'" -o build/bin/cke-k8s-proxy kubernetes/proxy/main/main.go

kvm-proto: vm/model/vm.pb.go
vm/model/vm.pb.go: vm/model/vm.proto
	protoc --gogo_out=plugins=grpc:./ --proto_path=./ -I./vendor -I/usr/local/protoc/include ./vm/model/vm.proto

kvm: kvm-proto
	@echo "===== build cke-vm-exec"
	CGO_ENABLED=1 GOOS=linux go build -tags "with_libvirt static_build" -ldflags "-X 'cke.BUILD=$(BUILD)' -X 'cke.VERSION=$(VERSION)'" -o build/bin/cke-vm-exec vm/main/main.go
	cp build/bin/cke-vm-exec html/build/exec/cke-vm-exec

#run_cmd:
#	@echo "===== build run_cmd"
#	CGO_ENABLED=0 GOOS=linux go build -ldflags "-s -w -extldflags '-static'" -o bin/client kubernetes/run_cmd/client.go
#	CGO_ENABLED=0 GOOS=linux go build -ldflags "-s -w -extldflags '-static'" -o bin/server kubernetes/run_cmd/server.go

images: scheduler-image wrapper-image

scheduler-image: scheduler k8s-exec
	@echo "===== build scheduler image"
	cd build/scheduler;make VERSION=$(VERSION)

wrapper-image: k8s-wrapper k8s-cke-proxy
	@echo "===== build wrapper image"
	cd build/wrapper.k8s; make VERSION=$(VERSION)

base_image:
ifeq ($(shell docker images | grep docker-alpine | grep v0\.0\.1),)
	docker load -i build/docker-alpine.tar
endif

build_test:
	CGO_ENABLED=0 GOOS=linux go build -ldflags "-s -w -extldflags '-static'" -o build/bin/cke-test test/main/main.go
	
test: build_test
	@echo "testing"
	#go test cke/scheduler
	#CGO_ENABLED=0 GOOS=linux go build -ldflags "-s -w -extldflags '-static'" -o build/bin/cke-test test/main/main.go
	build/bin/cke-test test/cases base_case,ha_case

help:
	@echo "make"
	@echo "   build all component of CKE"
	@echo ""
	@echo "make scheduler"
	@echo "   build cke-scheduler"
	@echo ""
	@echo "make k8s"
	@echo "   build cke-k8s-exec and cke-k8s-wrapper"
	@echo ""
	@echo "make k8s-exec"
	@echo "   build cke-k8s-exec"
	@echo ""
	@echo "make k8s-wrapper"
	@echo "   build cke-k8s-wrapper"
	@echo ""
	@echo "make cke k8s proxy"
	@echo "   build cke-k8s-proxy"
	@echo ""
	@echo "make kvm"
	@echo "   build cke-vm-exec"
	@echo ""
	@echo "make images"
	@echo "   build scheduler and wrapper docker images"
	@echo ""
	@echo "make scheduler-image"
	@echo "   build scheduler docker image"
	@echo ""
	@echo "make wrapper-image"
	@echo "   build wrapper docker image"
	@echo ""
	@echo "make test"
	@echo "   white-box testing"
