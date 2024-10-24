# Use the official Golang image as a parent image
FROM golang:1.20-alpine

# Set the working directory inside the container
WORKDIR /app

# Copy go mod and sum files
COPY go.mod go.sum ./

# Download all dependencies
RUN go mod download

# Copy the source code into the container
COPY src/ ./src/

# Build the application
RUN go build -o /interest_bot src/*.go

# Run the application
CMD ["/interest_bot"]
