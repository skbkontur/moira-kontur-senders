GIT_BRANCH := "master"
TAG_ALIAS := "master"
DATE := $(shell date '+%Y-%m-%d-%H-%M')

.PHONY: push_docker_image
push_docker_image:
	docker build --build-arg NOTIFIER_BRANCH=${GIT_BRANCH} -t skbkontur/moira-notifier:${TAG_ALIAS}.${DATE} .
	docker push skbkontur/moira-notifier:${TAG_ALIAS}.${DATE}
