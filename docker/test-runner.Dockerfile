FROM mcr.microsoft.com/powershell:7.5-debian-12 AS powershell

FROM golang:1.26-bookworm

COPY --from=powershell /opt/microsoft/powershell/7 /opt/microsoft/powershell/7
COPY --from=powershell /usr/lib/x86_64-linux-gnu/libicu*.so* /usr/lib/x86_64-linux-gnu/
RUN ln -s /opt/microsoft/powershell/7/pwsh /usr/local/bin/pwsh

WORKDIR /workspace

ENV ARDENTS_TEST_RUNTIME=container

ENTRYPOINT ["pwsh", "-NoLogo", "-NoProfile", "-File", "tests/run.ps1"]
