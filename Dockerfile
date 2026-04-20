FROM golang:1.25-alpine

RUN apk add --no-cache curl jq
WORKDIR /app

# 1. Only copy code, ensure no old modules exist
COPY . .
RUN rm -f go.mod go.sum

# 2. Initialize and force the specific YARPC version
RUN go mod init ghosttrace
RUN go get go.uber.org/yarpc@v1.88.1
RUN go get golang.org/x/sync/errgroup

# 3. Tidy will now see "yarpcerrors" in the code and find it in v1.88.1
RUN go mod tidy
RUN go build -o main .

CMD ["./main"]