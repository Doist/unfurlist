FROM public.ecr.aws/docker/library/golang:alpine AS builder
RUN apk add git
WORKDIR /app
ENV GOPROXY=https://proxy.golang.org CGO_ENABLED=0
COPY go.mod go.sum ./
RUN go mod download
COPY . ./
RUN go build -ldflags='-s -w' -o main ./cmd/unfurlist

FROM scratch
COPY --from=builder /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/
COPY --from=builder /app/main /bin/main
EXPOSE :8080
CMD ["/bin/main", "-pprof=''", "-listen=:8080"]
