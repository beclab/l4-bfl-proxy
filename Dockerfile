FROM golang:1.18 as builder

WORKDIR /workspace

ENV ksVersion=v3.3.0

# Copy the go project
COPY . bytetrade.io/web3os/l4-bfl-proxy/

RUN git clone https://github.com/kubesphere/kubesphere.git bytetrade.io/kubesphere && \
    cd bytetrade.io/kubesphere && \
    git checkout -b $ksVersion && \
    cd ../web3os/l4-bfl-proxy/ && \
    CGO_ENABLED=0 go build -a -o l4-bfl-proxy main.go

FROM bytetrade/openresty:1.25.3-otel
WORKDIR /
COPY --from=builder /workspace/bytetrade.io/web3os/l4-bfl-proxy/config/lua etc/nginx/lua
COPY --from=builder /workspace/bytetrade.io/web3os/l4-bfl-proxy/l4-bfl-proxy .
COPY fake-nginx /usr/local/bin/nginx
RUN chmod 755 /usr/local/bin/nginx

ENTRYPOINT ["/l4-bfl-proxy"]
