.PHONY: build docker

build:
	go build -o bin/registeel

docker:
	docker build -t rossedman/registeel-ctl:latest .
	docker build -t rossedman/registeel-api:latest ./api
	docker build -t rossedman/registeel-web:latest ./web

release:
	docker push rossedman/registeel-ctl:latest 
	docker push rossedman/registeel-api:latest
	docker push rossedman/registeel-web:latest

vuesetup:
	npm install -g @vue/cli
	npm install -g @vue/cli-service-global

