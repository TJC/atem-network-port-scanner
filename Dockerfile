FROM arm32v7/alpine
RUN apk update && apk add go
COPY ./* /root/
CMD cd /root && go build && cp atem-network-scanner /export/
