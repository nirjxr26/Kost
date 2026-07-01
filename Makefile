BINARY=kost
IMAGE=ghcr.io/nirjar/kost

build:
	go build -o $(BINARY) ./cmd/kost/

image:
	docker build -t $(IMAGE):latest .

push: image
	docker push $(IMAGE):latest

deploy:
	kubectl apply -f deploy/rbac.yaml -f deploy/configmap.yaml -f deploy/deployment.yaml -f deploy/service.yaml

undeploy:
	kubectl delete -f deploy/rbac.yaml -f deploy/configmap.yaml -f deploy/deployment.yaml -f deploy/service.yaml

.PHONY: build image push deploy undeploy
