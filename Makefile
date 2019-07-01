all: handler push

organization_name := piotrkubisa
repository_name := nexmo-consumer
repository_path := github.com/$(organization_name)/$(repository_name)

cmd_name ?= $(repository_name)
binary_name ?= handler

stage_name ?= canary
aws_profile ?= default
aws_region ?= eu-west-1
s3_bucket ?= lambda-fn-$(aws_region)
cf_lambda_template ?= ./cloudformation/lambda.yml
cf_packaged_template ?= packaged.yml

deps:
	glide install
.PHONY: deps

handler: build
.PHONY: handler

build:
	$(call blue, "Building Linux binary using Docker...")
	docker run \
		--rm \
		-it \
		-v "$(CURDIR)":/gopath/src/$(repository_path) \
		-w "/gopath/src/$(repository_path)" \
		-e "GOPATH=/gopath" \
		golang:1 \
		sh -c "binary_name=$(binary_name) cmd_name=$(cmd_name) make compile"
.PHONY: build

compile:
	$(call blue, "Compile binary with the Go compiler...")
	GOOS=linux GOARCH=amd64 CGO_ENABLED=0 \
	go build -v -o ./dist/$(binary_name) \
		-ldflags "-s -w -X main.BuildVersion=$(build_version) -X main.BuildDate=$(build_date)" \
		./cmd/$(cmd_name)
.PHONY: compile

push: validate lambda
.PHONY: push

validate:
	$(call blue, "Validating CloudFormation template...")
	aws cloudformation validate-template --template-body file://$(cf_lambda_template)
.PHONY: validate

lambda: package deploy
.PHONY: lambda

package:
	$(call blue, "Packaging SAM application...")
	aws cloudformation package \
		--profile $(aws_profile) \
		--template-file $(cf_lambda_template) \
		--s3-bucket $(s3_bucket) \
		--s3-prefix $(repository_name)-$(stage_name) \
		--output-template-file $(cf_packaged_template)
.PHONY: package

deploy:
	$(call blue, "Deploying SAM application...")
	aws cloudformation deploy \
		--profile $(aws_profile) \
		--region $(aws_region) \
		--template-file $(cf_packaged_template) \
		--stack-name $(repository_name)-$(stage_name)-lambda \
		--parameter-overrides \
			GitHubRepositoryName=$(repository_name) \
			Stage=$(stage_name) \
		--tags Environment=$(stage_name) \
		--capabilities CAPABILITY_IAM
.PHONY: deploy

clean:
	@rm -rf \
		$(cf_packaged_template) \
		$(lambda_artifact) \
		dist
.PHONY: clean
