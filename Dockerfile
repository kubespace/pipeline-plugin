ARG BASEIMAGE=kubespace/distroless-static:latest
FROM $BASEIMAGE

COPY pipeline-plugin /

CMD ["/pipeline-plugin"]
