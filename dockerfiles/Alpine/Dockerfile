FROM golang:1.22.0-alpine3.19
WORKDIR /root
RUN \
  apk update && \
  apk add git build-base && \
  git clone https://github.com/whildbear/kubernetes-cluster-autoscaler.git && \
  cd kubernetes-cluster-autoscaler && \
  go mod download github.com/gophercloud/gophercloud && \
  go mod download github.com/gophercloud/utils && \
  go mod download golang.org/x/crypto && \
  go mod tidy && \
  go build -o autoscaler  cmd/main.go


FROM golang:1.22.0-alpine3.19
WORKDIR /go/bin
COPY --from=0 /root/kubernetes-cluster-autoscaler/autoscaler .
CMD ["./autoscaler"]
