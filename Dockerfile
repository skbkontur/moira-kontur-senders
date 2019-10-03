FROM golang:1.13.1 as builder

ARG NOTIFIER_BRANCH=master

RUN go get -v github.com/AlexAkulov/go-humanize

RUN go get github.com/moira-alert/moira
WORKDIR /go/src/github.com/moira-alert/moira
RUN git checkout $NOTIFIER_BRANCH

COPY ./senders/ /go/src/github.com/moira-alert/moira/senders/kontur
RUN sed -i "s/^[[:space:]]\+\/\///g" ./notifier/registrator.go

RUN CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -o build/notifier github.com/moira-alert/moira/cmd/notifier


FROM alpine

RUN apk add --no-cache ca-certificates && update-ca-certificates

COPY --from=builder /go/src/github.com/moira-alert/moira/pkg/notifier/notifier.yml /etc/moira/notifier.yml
COPY --from=builder /go/src/github.com/moira-alert/moira/build/notifier /usr/bin/notifier
COPY --from=builder /usr/local/go/lib/time/zoneinfo.zip /usr/local/go/lib/time/

ENTRYPOINT ["/usr/bin/notifier"]
