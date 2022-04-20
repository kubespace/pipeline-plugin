ARG BASEIMAGE=registry.cn-hangzhou.aliyuncs.com/kubespace/pipeline-plugin-base:v2
FROM $BASEIMAGE

COPY pipeline-plugin /

CMD ["/pipeline-plugin"]
