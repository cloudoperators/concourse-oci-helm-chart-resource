FROM golang:1.22 as build

WORKDIR /concourse-oci-helm-chart-resource
COPY . .
RUN make build

FROM alpine AS run

LABEL org.opencontainers.image.source = "https://github.com/cloudoperators/concourse-oci-helm-chart-resource"

# Required by concourse resource. Copy explicitly.
COPY --from=build /concourse-oci-helm-chart-resource/bin/check /opt/resource/check
RUN chmod +x /opt/resource/check

COPY --from=build /concourse-oci-helm-chart-resource/bin/in /opt/resource/in
RUN chmod +x /opt/resource/in

COPY --from=build /concourse-oci-helm-chart-resource/bin/out /opt/resource/out
RUN chmod +x /opt/resource/out
