# Use the official Golang image as a parent image
FROM golang:1.22-alpine

# Set the working directory inside the container
WORKDIR /app

# Install required build dependencies
RUN apk add --no-cache gcc musl-dev sqlite-dev

# Set CGO_ENABLED for sqlite support
ENV CGO_ENABLED=1

# Copy go mod and sum files
COPY go.mod go.sum ./

# Download all dependencies
RUN go mod download

# Copy the source code into the container
COPY src/ ./src/

EXPOSE 8080

CMD ["go", "run", "./src"]
