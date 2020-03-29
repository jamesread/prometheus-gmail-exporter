#!/usr/bin/python3
"""
Checks gmail labels for unread messages and exposes the counts via prometheus.
"""

import os
import sys
import sched
from time import time, sleep
import logging
from functools import lru_cache

import httplib2
import configargparse

from prometheus_client import start_http_server, Gauge

from googleapiclient import discovery
from oauth2client import client
from oauth2client.file import Storage

def get_file_path(filename):
    config_dir = os.path.join(os.path.expanduser("~"), ".prometheus-gmail-exporter")

    if not os.path.exists(config_dir):
        os.mkdir(config_dir)

    return os.path.join(config_dir, filename)

def get_credentials():
    """Gets valid user credentials from storage.

    If nothing has been stored, or if the stored credentials are invalid,
    the OAuth2 flow is completed to obtain the new credentials.
    """

    if not os.path.exists(args.clientSecretFile):
        logging.fatal("Client secrets file does not exist: %s . You probably need to download this from the Google API console.", args.clientSecretFile)
        sys.exit()

    credentials_path = args.credentialsPath

    store = Storage(credentials_path)
    credentials = store.get()

    if not credentials or credentials.invalid:
        scopes = 'https://www.googleapis.com/auth/gmail.readonly https://www.googleapis.com/auth/gmail.metadata'
        
        flow = client.flow_from_clientsecrets(args.clientSecretFile, scopes)
        flow.user_agent = 'prometheus-gmail-exporter'

        credentials = run_flow(flow, store)

        logging.info("Storing credentials to %s", credentials_path)

    return credentials

def run_flow(flow, store):
    flow.redirect_uri = client.OOB_CALLBACK_URN
    authorize_url = flow.step1_get_authorize_url()
    
    logging.info("Go and authorize at: %s", authorize_url)
    code = input('Enter code:').strip()

    try:
        credential = flow.step2_exchange(code, http=None)
    except client.FlowExchangeError as e:
        logging.fatal("Auth failure: %s", str(e))
        sys.exit(1)

    store.put(credential)
    credential.set_store(store)

    return credential

@lru_cache(maxsize=1)
def get_labels():
    """
    Note that this func is cached (lru_cache) and will only run once.
    """

    logging.info("Getting metadata about labels")

    labels = []

    if len(args.labels) == 0:
        logging.warning("No labels specified, assuming all labels. If you have a lot of labels in your inbox you could hit API limits quickly.")
        results = GMAIL_CLIENT.users().labels().list(userId='me').execute()

        labels = results.get('labels', [])
    else:
        for label in args.labels:
            labels.append({'id': label})

    if not labels:
        logging.info('No labels found.')
        sys.exit()

    return labels

gauge_collection = {}

def get_gauge_for_label(name, desc):
    if name not in gauge_collection:
        gauge = Gauge(name, desc)
        gauge_collection[name] = gauge

    return gauge_collection[name]

def update_gauages_from_gmail(*unused_arguments_needed_for_scheduler):
    logging.info("Updating gmail metrics ")

    for label in get_labels():
        try: 
            label_info = GMAIL_CLIENT.users().labels().get(id=label['id'], userId='me').execute()

            gauge = get_gauge_for_label(label_info['id'] + '_total', label_info['name']  + ' Total')
            gauge.set(label_info['threadsTotal'])

            gauge = get_gauge_for_label(label_info['id'] + '_unread', label_info['name'] + ' Unread')
            gauge.set(label_info['threadsUnread'])
        except Exception as e:
            # eg, if this script is started with a label that exists, that is then deleted
            # after startup, 404 exceptions are thrown.
            #
            # Occsionally, the gmail API will throw garbage, too. Hence the try/catch.
            logging.error("Error: %s", e)

def get_gmail_client():
    credentials = get_credentials()
    http_client = credentials.authorize(httplib2.Http())
    return discovery.build('gmail', 'v1', http=http_client)

def main():
    logging.getLogger().setLevel(20)

    global GMAIL_CLIENT
    GMAIL_CLIENT = get_gmail_client()

    logging.info("prometheus-gmail-exporter started on port %d", args.promPort)
    start_http_server(args.promPort)

    update_gauages_from_gmail() # So we don't have to wait for the first delay

    scheduler = sched.scheduler(time, sleep)
    scheduler.enter(args.updateDelaySeconds, 1, update_gauages_from_gmail)
    scheduler.run()

if __name__ == '__main__':
    global args
    parser = configargparse.ArgumentParser(default_config_files=[get_file_path('prometheus-gmail-exporter.cfg'), "/etc/prometheus-gmail-exporter.cfg"])
    parser.add_argument('labels', nargs='*', default=[])
    parser.add_argument('--clientSecretFile', default=get_file_path('client_secret.json'))
    parser.add_argument('--credentialsPath', default=get_file_path('login_cookie.dat'))
    parser.add_argument("--updateDelaySeconds", type=int, default=300)
    parser.add_argument("--promPort", type=int, default=8080)
    args = parser.parse_args()

    try:
        main()
    except KeyboardInterrupt:
        print("\n") # Most terminals print a Ctrl+C message as well. Looks ugly with our log.
        logging.info("Ctrl+C, bye!")
