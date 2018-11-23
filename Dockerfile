FROM golang:1.11-alpine AS builder

# Install git
RUN apk add --update --no-cache git

# Download dependencies
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download

# Build executable files
ENV CGO_ENABLED 0
COPY . ./
RUN go build -tags netgo -ldflags "-w" .

FROM alpine

# Copy binaries
COPY --from=builder /src/layercake /usr/local/bin/layercake
ENTRYPOINT ["layercake"]