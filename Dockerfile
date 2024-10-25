# Use the official Golang image as a parent image
FROM golang:1.22-alpine

# Set the working directory inside the container
WORKDIR /app

# Copy go mod and sum files
COPY go.mod go.sum ./

RUN apk add --no-cache gcc musl-dev

ARG CGO_ENABLED=1

# Download all dependencies
RUN go mod download

# Copy the source code into the container
COPY src/ ./src/

# Build the application
# RUN go build -o /interest_bot src/*.go

# Run the application
# CMD ["/interest_bot"]

EXPOSE 8080

CMD ["go", "run", "./src"]
