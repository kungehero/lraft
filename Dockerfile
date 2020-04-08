FROM alpine:latest
COPY cmd/lraft/lraft /bin/usr/lraft
VOLUME [ "/raft" ]
ENTRYPOINT ./bin/usr/lraft 
LABEL Name=lraft Version=0.0.1