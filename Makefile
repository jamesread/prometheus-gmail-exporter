lint:
	pylint-3 gmail-exporter.py

docker:
	docker build . -t docker.io/jamesread/prometheus-gmail-exporter
