wrapper_image_name=reg.mg.hcbss/open/cke-k8s-wrapper

.PHONY: all scheduler k8s k8s-proto k8s-exec k8s-wrapper kvm images scheduler-image wrapper-image base_image test help

wrapper-image: base_image 
	rm docker/files -rf; mkdir docker/files
	cp files/lib64 docker/files/lib64 -rf
	cp files/usr docker/files/ -rf
	cp files/opt docker/files/ -rf
	cp files/etc docker/files/ -rf
	cp ../bin/cke-k8s-wrapper docker/files/usr/local/bin/
	cp ../bin/cke-k8s-proxy docker/files/usr/local/bin/
	chmod 755 docker/files/lib64/*
	chmod 755 docker/files/usr/local/bin/*
	chmod 755 docker/files/usr/bin/*
	chmod 755 docker/files/opt/cni/bin/*
	dos2unix docker/files/usr/local/bin/*.sh
ifneq ($(shell docker images|grep $(wrapper_image_name) | grep v$(VERSION)),)
	docker images|grep $(wrapper_image_name) | grep v$(VERSION) | awk '{print $$3}' | xargs -r docker rmi -f
endif
	cd docker; docker build -t $(wrapper_image_name):v$(VERSION) -f ./Dockerfile.k8s.wrapper .
	docker push $(wrapper_image_name):v$(VERSION)

base_image:
ifeq ($(shell docker images | grep docker-alpine | grep v0\.0\.5),)
	docker load -i docker-alpine-v0.0.5.tar
endif

