FROM alpine:latest

RUN apk add --update alpine-sdk linux-headers git zlib-dev openssl-dev gperf php php-ctype cmake make musl-dev go

# Build tdlib
RUN git clone https://github.com/tdlib/td.git
RUN mkdir td/build
WORKDIR td/build
RUN cmake -DCMAKE_BUILD_TYPE=Release ..
RUN cmake --build .
RUN make install

# Build app
ENV GOPATH=/go
ADD . /go/src/app
WORKDIR /go/src/app
RUN go get -v
RUN go build -o /app

WORKDIR /
CMD /app