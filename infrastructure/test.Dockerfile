FROM golang:1.27.3-alpine
RUN apk add --no-cache gcc musl-dev
WORKDIR /src
COPY backend/go.mod backend/go.sum ./
RUN go mod download
COPY backend/ ./
ENV CGO_ENABLED=1
CMD ["go", "test", "-race", "-count=1", "-v", "./..."]
