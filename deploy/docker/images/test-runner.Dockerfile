FROM mcr.microsoft.com/powershell:7.5-debian-12@sha256:7ab5bd5ca6f95a3351fa0c6a1205237d57048c94542355aab55519a0861a9b25 AS powershell

FROM docker.io/library/golang:1.26.5-bookworm@sha256:3f6236bd765f898a2a3c2946112b04097814c4529d44534674700cd07b9c6b4c

COPY --from=powershell /opt/microsoft/powershell/7 /opt/microsoft/powershell/7
COPY --from=powershell /usr/lib/x86_64-linux-gnu/libicu*.so* /usr/lib/x86_64-linux-gnu/
RUN ln -s /opt/microsoft/powershell/7/pwsh /usr/local/bin/pwsh

WORKDIR /workspace

ENV ARDENTS_TEST_RUNTIME=container

ENTRYPOINT ["pwsh", "-NoLogo", "-NoProfile", "-File", "tests/run.ps1"]
