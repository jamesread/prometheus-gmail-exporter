FROM fedora

RUN dnf -y update && \
	dnf -y install python3-google-auth-oauthlib python3-configargparse python3-httplib2 python3-oauth2client python3-pyyaml python3-google-api-client.noarch python3-prometheus_client.noarch && \
	dnf clean all

COPY gmail-exporter.py /usr/local/sbin/gmail-exporter

ENTRYPOINT [ "/usr/local/sbin/gmail-exporter" ]

EXPOSE 8080
