#!/usr/bin/env python3
"""
Checks gmail labels for unread messages and exposes the counts via prometheus.
"""

import os
import sys
from time import sleep
import logging
from functools import lru_cache

import configargparse

from prometheus_client import start_http_server, Gauge

from googleapiclient import discovery
from google_auth_oauthlib.flow import InstalledAppFlow
from google.oauth2.credentials import Credentials

GMAIL_CLIENT = None
THREAD_SENDER_CACHE = {}

def get_homedir_filepath(filename):
    config_dir = os.path.join(os.path.expanduser("~"), ".prometheus-gmail-exporter")

    if not os.path.exists(config_dir):
        os.mkdir(config_dir)

    return os.path.join(config_dir, filename)

def get_credentials():
    """Gets valid user credentials from storage.

    If nothing has been stored, or if the stored credentials are invalid,
    the OAuth2 flow is completed to obtain the new credentials.
    """

    SCOPES = 'https://www.googleapis.com/auth/gmail.readonly '

    while not os.path.exists(args.clientSecretFile):
        logging.fatal("Client secrets file does not exist: %s . You probably need to download this from the Google API console.", args.clientSecretFile)
        sleep(10)

    credentials = None

    if os.path.exists(args.credentialsPath):
        credentials = Credentials.from_authorized_user_file(args.credentialsPath, SCOPES)

    if not credentials or not credentials.valid:
        flow = InstalledAppFlow.from_client_secrets_file(args.clientSecretFile, SCOPES)
        flow.user_agent = 'prometheus-gmail-exporter'

        credentials = flow.run_local_server(port=args.oauthBindPort, bind_addr = args.oauthBindAddr, host = args.oauthHost)
        #credentials = flow.run_local_server()

        logging.info("Storing credentials to %s", args.credentialsPath)

    with open(args.credentialsPath, 'w', encoding='utf8') as token:
        token.write(credentials.to_json())


    return credentials

def run_flow_oob_deprecated(flow):
    #flow.redirect_uri = client.OOB_CALLBACK_URN
    flow.run_local_server(port=0)
    #authorize_url = flow.step1_get_authorize_url()

    #logging.info("Go and authorize at: %s", authorize_url)

    if sys.stdout.isatty():
        code = input('Enter code:').strip()
    else:
        logging.info("Waiting for code at %s", get_homedir_filepath('auth_code'))

        while True:
            try:
                if os.path.exists(get_homedir_filepath('auth_code')):
                    with open(get_homedir_filepath('auth_code'), 'r', encoding='utf8') as auth_code_file:
                        code = auth_code_file.read()
                        break

            except Exception as e:
                logging.critical(e)

            sleep(10)

    try:
        credential = flow.step2_exchange(code, http=None)
    except Exception as e:
        logging.fatal("Auth failure: %s", str(e))
        sys.exit(1)

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
        logging.info('Using labels: %s ', args.labels)

        for label in args.labels:
            labels.append({'id': label})

    if not labels:
        logging.info('No labels found.')
        sys.exit()

    return labels

gauge_collection = {}

def get_gauge_for_label(name, desc, labels = None):
    if labels is None:
        labels = []

    if name not in gauge_collection:
        gauge = Gauge('gmail_' + name, desc, labels)
        gauge_collection[name] = gauge

    return gauge_collection[name]

def update_gauages_from_gmail(*unused_arguments_needed_for_scheduler):
    logging.info("Updating gmail metrics - started")

    for label in get_labels():
        try:
            label_info = GMAIL_CLIENT.users().labels().get(id=label['id'], userId='me').execute()

            gauge = get_gauge_for_label(label_info['id'] + '_total', label_info['name']  + ' Total')
            gauge.set(label_info['threadsTotal'])

            gauge = get_gauge_for_label(label_info['id'] + '_unread', label_info['name'] + ' Unread')
            gauge.set(label_info['threadsUnread'])

            if label['id'] in args.labelsSenderCount:
                update_sender_gauges_for_label(label_info['id'])

        except Exception as e:
            # eg, if this script is started with a label that exists, that is then deleted
            # after startup, 404 exceptions are thrown.
            #
            # Occsionally, the gmail API will throw garbage, too. Hence the try/catch.
            logging.error("Error: %s", e)

    logging.info("Updating gmail metrics - complete")

def get_first_message_sender(thread):
    if thread is None or thread['messages'] is None:
        return "unknown-thread-no-messages"

    firstMessage = thread['messages'][0]

    for header in firstMessage['payload']['headers']:
        if header['name'] == 'From':
            return header['value']

    return "unknown-no-from"

def get_all_threads_for_label(labelId):
    logging.info("get_all_threads_for_label - this method can be expensive: %s", str(labelId))

    response = GMAIL_CLIENT.users().threads().list(userId = 'me', labelIds = [labelId], q = "is:unread").execute()

    threads = []

    logging.info("get_all_threads_for_label - result size estimate: %s", str(response['resultSizeEstimate']))

    if "threads" in response:
        threads.extend(response['threads'])

    while "nextPageToken" in response:
        page_token = response['nextPageToken']
        response = GMAIL_CLIENT.users().threads().list(userId = 'me', labelIds = [labelId], pageToken = page_token, q = "is:unread").execute()
        threads.extend(response['threads'])

        logging.info("Getting more threads for label %s: %s", labelId, str(len(threads)))

    return threads

def get_thread_messages(thread):
    logging.info("Fetching thread messages for %s", str(thread['id']))

    res = GMAIL_CLIENT.users().threads().get(userId = 'me', id = thread['id'], format = "metadata").execute()

    thread['messages'] = res['messages']

    return thread

def update_sender_gauges_for_label(label):
    senderCounts = {}

    for thread in get_all_threads_for_label(label):
        if thread['id'] not in THREAD_SENDER_CACHE:
            thread = get_thread_messages(thread)

            THREAD_SENDER_CACHE[thread['id']] = get_first_message_sender(thread)

        sender = THREAD_SENDER_CACHE[thread['id']]

        if sender not in senderCounts:
            senderCounts[sender] = 0

        senderCounts[sender] += 1

    for sender, messageCount in senderCounts.items():
        g = get_gauge_for_label(label + '_sender', 'Label sender info', ['sender'])
        g.labels(sender=sender).set(messageCount)

def get_gmail_client():
    return discovery.build('gmail', 'v1', credentials = get_credentials())

def infinate_update_loop():
    while True:
        update_gauages_from_gmail()
        sleep(args.updateDelaySeconds)

def main():
    global GMAIL_CLIENT
    GMAIL_CLIENT = get_gmail_client()

    logging.info("Got gmail client successfully")
    
    start_http_server(args.promPort)

    logging.info("Prometheus started on port %d", args.promPort)

    if args.daemonize:
        infinate_update_loop()
    else:
        update_gauages_from_gmail()

if __name__ == '__main__':
    global args
    parser = configargparse.ArgumentParser(default_config_files=[
        get_homedir_filepath('prometheus-gmail-exporter.cfg'),
        get_homedir_filepath('prometheus-gmail-exporter.yaml'),
        "/etc/prometheus-gmail-exporter.cfg",
        "/etc/prometheus-gmail-exporter.yaml",
    ], config_file_parser_class=configargparse.YAMLConfigFileParser)

    parser.add_argument('--labels', nargs='*', default=[])
    parser.add_argument("--labelsSenderCount", nargs='*', default=[])
    parser.add_argument('--clientSecretFile', default=get_homedir_filepath('client_secret.json'))
    parser.add_argument('--credentialsPath', default=get_homedir_filepath('login_cookie.dat'))
    parser.add_argument("--updateDelaySeconds", type=int, default=300)
    parser.add_argument("--oauthHost", type=str, default="localhost")
    parser.add_argument("--oauthBindAddr", type=str, default="0.0.0.0")
    parser.add_argument("--oauthBindPort", type=int, default=9090)
    parser.add_argument("--promPort", type=int, default=8080)
    parser.add_argument("--daemonize", action='store_true')
    parser.add_argument("--logLevel", type=int, default = 20)
    args = parser.parse_args()

    logging.getLogger().setLevel(args.logLevel)
    logging.info("prometheus-gmail-exporter is starting up.")
    logging.info("args (from config, and flags): %s", args)
    logging.info("UID: %s", os.getuid())
    logging.info("Home directory: %s", os.getenv("HOME"))

    try:
        main()
    except KeyboardInterrupt:
        print("\n") # Most terminals print a Ctrl+C message as well. Looks ugly with our log.
        logging.info("Ctrl+C, bye!")
