FROM golang:1.21
ADD . /src
ENV DEBIAN_FRONTEND noninteractive
RUN apt update && apt upgrade -y && apt install -y protoc-gen-go protobuf-compiler build-essential
WORKDIR /src
RUN make all 
RUN mv fete-node /usr/local/bin && chmod +x /usr/local/bin/fete-node
