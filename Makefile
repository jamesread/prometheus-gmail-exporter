lint:
	pylint-3 gmail-exporter.py

buildah:
	buildah bud -t docker.io/jamesread/prometheus-gmail-exporter

docker:
	docker build . -t docker.io/jamesread/prometheus-gmail-exporter
