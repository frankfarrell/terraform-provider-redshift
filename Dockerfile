# Allow selecting terraform release, and use "latest" if no build argument is specified
ARG TERRAFORM_RELEASE="latest"

# First step, use golang image for build of plugin
FROM golang:1.11.5-alpine3.9 AS builder

# Allow specifying provider release version
ARG TERRAFORM_PROVIDER_REDSHIFT_RELEASE="v0.0.2"

# Add build dependencies && create terraform plugins dir
RUN apk add --update git && \
    go get github.com/mitchellh/gox && \
    mkdir -p ~/.terraform.d/plugins/linux_amd64/

# Fetch source code of redshift provider and build it
RUN go get github.com/frankfarrell/terraform-provider-redshift && \
    cd $GOPATH/src/github.com/frankfarrell/terraform-provider-redshift && \
    gox -osarch="linux/amd64" -output="${HOME}/.terraform.d/plugins/{{.OS}}_{{.Arch}}/terraform-provider-redshift_${TERRAFORM_PROVIDER_REDSHIFT_RELEASE}" .

# Second step, use official terraform image
FROM hashicorp/terraform:${TERRAFORM_RELEASE}

# Create plugin directory
RUN mkdir -p /root/.terraform.d/plugins/

# Copy compiled plugin from first step
COPY --from=builder /root/.terraform.d/plugins/* /root/.terraform.d/plugins/
