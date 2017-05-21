FROM        quay.io/prometheus/busybox:latest
MAINTAINER  Alexander Fahlke <alexander.fahlke+prometheus@gmail.com>

COPY namenode_exporter /bin/namenode_exporter

ENTRYPOINT ["/bin/namenode_exporter"]
EXPOSE     9779
