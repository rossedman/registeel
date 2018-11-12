FROM golang:1.10-stretch as builder
WORKDIR /go/src/github.com/rossedman/registeel
COPY . ./
RUN make build

FROM golang:1.10-stretch
COPY --from=builder /go/src/github.com/rossedman/registeel/bin/registeel /usr/local/bin
ENTRYPOINT ["/usr/local/bin/registeel"]
