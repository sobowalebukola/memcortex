# Use Go 1.24 (or 1.25 if you want latest)
FROM golang:1.24-alpine

# Install git (needed for some modules)
RUN apk add --no-cache git

# Set working directory
WORKDIR /app

# Copy go.mod and go.sum first
COPY go.mod go.sum ./

# Download dependencies
RUN go mod download

# Copy the rest of the project
COPY . .

# Expose the port your server listens on
EXPOSE 8080

# Run the server
CMD ["go", "run", "./cmd/server"]
