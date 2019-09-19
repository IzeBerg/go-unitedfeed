FROM izeberg/tdlib:latest

RUN apk add --update go git mercurial musl-dev

# Build app
ENV GOPATH=/go
ADD . /go/src/app
WORKDIR /go/src/app
RUN go get -v
RUN go build -o /app

WORKDIR /
CMD /app