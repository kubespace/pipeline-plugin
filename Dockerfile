ARG BASEIMAGE=kubespace/busybox:v1.33.1
FROM $BASEIMAGE

COPY pipeline-plugin /

CMD ["/pipeline-plugin"]
