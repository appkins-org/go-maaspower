FROM scratch
COPY go-maaspower /
ENTRYPOINT ["/go-maaspower"]
