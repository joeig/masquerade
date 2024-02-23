FROM golang:1.22 as build

RUN adduser \
    --disabled-password \
    --gecos "" \
    --home "/nonexistent" \
    --shell "/sbin/nologin" \
    --no-create-home \
    --uid 10000 \
    runuser

WORKDIR /go/src/app
COPY . .

ENV CGO_ENABLED=0

RUN go install -v ./cmd/masquerade/...

FROM scratch

COPY --from=build /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/
COPY --from=build /etc/passwd /etc/passwd
COPY --from=build /etc/group /etc/group
COPY --from=build /go/bin/ /bin/

USER runuser:runuser
EXPOSE 8493
EXPOSE 9091
WORKDIR /bin

CMD ["masquerade"]
