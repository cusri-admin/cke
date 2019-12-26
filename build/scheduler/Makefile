scheduler_image_name=reg.mg.hcbss/open/cke-scheduler

.PHONY: scheduler-image

scheduler-image: base_image
	rm docker/files -rf; mkdir docker/files
	cp default_cfg docker/files/ -rf
	cp reg.mg.hcbss docker/files/ -rf
	cp harbor.dcos.xixian.unicom.local docker/files/ -rf
	mkdir -p docker/files/html/build/exec docker/files/html/build/ui
	cp ../bin/cke-scheduler docker/files/ -rf
	cp ../../html/build/exec/* docker/files/html/build/exec/ -rf
	cp ../../html/build/ui/* docker/files/html/build/ui/ -rf
ifneq ($(shell docker images|grep $(scheduler_image_name) | grep v$(VERSION)),)
	docker images|grep $(scheduler_image_name) | grep v$(VERSION) | awk '{print $$3}' | xargs -r docker rmi -f
endif
	cd docker; docker build -t $(scheduler_image_name):v$(VERSION) -f ./Dockerfile.k8s.scheduler .
	docker push $(scheduler_image_name):v$(VERSION)

base_image:
ifeq ($(shell docker images | grep scheduler-alpine | grep v0\.0\.1),)
	docker load -i scheduler-alpine-v0.0.1.tar
endif
