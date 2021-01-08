
# Image URL to use all building/pushing image targets
IMG ?= leiwanjun/fluent-bit:v1.6.9
AMD64 ?= -amd64

all: image

# Build the docker image for amd64 and arm64
image:
	docker buildx build --push --platform linux/amd64,linux/arm64 -f Dockerfile . -t ${IMG}

# Build all docker images for amd64
image-amd64: build-op-amd64 build-migtator-amd64
	docker build -f Dockerfile . -t ${IMG}${AMD64}

# Push the docker image
push-amd64:
	docker push ${IMG}${AMD64}
