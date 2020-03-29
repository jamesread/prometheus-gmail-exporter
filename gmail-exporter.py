#!/usr/bin/python3

import httplib2
import os
import json
import sys

from prometheus_client import start_http_server, Gauge

import sched, time

from googleapiclient import discovery, http
from oauth2client import client
from oauth2client import tools
from oauth2client.file import Storage
import time
from datetime import datetime
import time

import configargparse

global args
parser = configargparse.ArgumentParser(parents=[tools.argparser])
parser.add_argument('labels', nargs = '*', default = []);
parser.add_argument('--clientSecretFile', default = '/etc/prometheus-email-exporter.json')
parser.add_argument("--login", action = 'store_true')
parser.add_argument("--updateDelaySeconds", type = int, default = 300)
parser.add_argument("--promPort", type = int, default = 8080)
args = parser.parse_args();

SCOPES = 'https://www.googleapis.com/auth/gmail.readonly https://www.googleapis.com/auth/gmail.metadata'
APPLICATION_NAME = 'prometheus-gmail-exporter'

def get_credential_file():
    home_dir = os.path.expanduser('~')
    credential_dir = os.path.join(home_dir, '.credentials')

    if not os.path.exists(credential_dir):
        os.makedirs(credential_dir)

    credentialFile = os.path.join(credential_dir, 'prometheus-gmail-exporter.json')

    return credentialFile

def get_credentials():
    """Gets valid user credentials from storage.

    If nothing has been stored, or if the stored credentials are invalid,
    the OAuth2 flow is completed to obtain the new credentials.

    Returns:
        Credentials, the obtained credential.
    """

    credential_path = get_credential_file();

    store = Storage(credential_path)
    credentials = store.get()

    if not credentials or credentials.invalid:
        flow = client.flow_from_clientsecrets(args.clientSecretFile, SCOPES)
        flow.user_agent = APPLICATION_NAME

        credentials = tools.run_flow(flow, store)

        print('Storing credentials to ' + credential_path)

    return credentials

def getLabels(service):
    labels = []

    if len(args.labels) == 0:
        results = service.users().labels().list(userId='me').execute()

        labels = results.get('labels', [])
    else:
        for label in args.labels:
            labels.append({'id': label})

    if not labels:
        print('No labels found.')
        sys.exit();

    return labels;

counters = {}

def getCounter(name, desc):
    if name not in counters:
        c = Gauge(name, desc);
        counters[name] = c;

    return counters[name]

def buildMetrics(*args):
    print("Building metrics");

    for label in labels:
        try: 
            label_info = service.users().labels().get(id = label['id'], userId = 'me').execute()
            labelId = label_info['id']

            c = getCounter(labelId + '_total')
            c.set(label_info['threadsTotal'])

            c = getCounter(labelId + '_unread')
            c.set(label_info['threadsUnread'])
        except Exception as e: 
            # eg, if this script is started with a label that exists, that is then deleted
            # after startup, 404 exceptions are thrown.
            #
            # Occsionally, the gmail API will throw garbage, too. Hence the try/catch.
            print("Error!", e)

def getService():
    credentials = get_credentials()
    http = credentials.authorize(httplib2.Http())
    service = discovery.build('gmail', 'v1', http=http)

    return service

def main():
    start_http_server(args.promPort)

    global service
    service = getService();
 
    global labels
    labels = getLabels(service)

    buildMetrics(); # So we don't have to wait for the first delay

    s = sched.scheduler(time.time, time.sleep)
    s.enter(args.updateDelaySeconds, 1, buildMetrics)
    s.run()

    buildMetrics(service, labels);

if __name__ == '__main__':
    main()
