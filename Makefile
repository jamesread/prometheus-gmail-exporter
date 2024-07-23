lint:
	pylint-3 gmail-exporter.py

buildah:
	buildah bud -t docker.io/jamesread/prometheus-gmail-exporter

docker:
	docker build . -t docker.io/jamesread/prometheus-gmail-exporter

devcerts:
	openssl req -x509 -newkey rsa:4096 -keyout key.pem -out cert.pem -days 365 -nodes
