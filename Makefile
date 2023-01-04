default:
	go build github.com/jamesread/prometheus-gmail-exporter/cmd/prometheus-gmail-exporter

lint:
	pylint-3 gmail-exporter.py

docker:
	docker build . -t docker.io/jamesread/prometheus-gmail-exporter
