#!/usr/bin/env python3
"""
Checks gmail labels for unread messages and exposes the counts via prometheus.
"""

import os
import sys
from time import sleep
import logging
from functools import lru_cache
from threading import Thread

import configargparse
import yaml

from prometheus_client import make_wsgi_app, Gauge

from flask import Flask, Response, request, session

from werkzeug.middleware.dispatcher import DispatcherMiddleware

import waitress

from googleapiclient import discovery
from google_auth_oauthlib.flow import Flow
from google.oauth2.credentials import Credentials

GMAIL_CLIENT = None
THREAD_SENDER_CACHE = {}
READINESS = "STARTUP"
SCOPES = 'https://www.googleapis.com/auth/gmail.readonly '

authComplete = False
flaskapp = Flask('prometheus-gmail-exporter')
flaskapp.secret_key = os.urandom(24)
gauge_collection = {}
args = None

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

    set_readiness("GET_CREDENTIALS")

    while not os.path.exists(args.clientSecretFile):
        logging.fatal("Client secrets file does not exist: %s . You probably need to download this from the Google API console.", args.clientSecretFile)
        sleep(10)

    credentials = None

    if os.path.exists(args.credentialsPath):
        logging.info("Loading credentials from %s", args.credentialsPath)
        credentials = Credentials.from_authorized_user_file(args.credentialsPath, SCOPES)

    return credentials

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

def get_gauge_for_label(name, desc, labels = None):
    if labels is None:
        labels = []

    if name not in gauge_collection:
        gauge = Gauge('gmail_' + name, desc, labels)
        gauge_collection[name] = gauge

    return gauge_collection[name]

def get_gauge_for_query(name):
    print(gauge_collection)

    if name not in gauge_collection:
        gauge = Gauge('gmail_' + name, name, [])
        gauge_collection[name] = gauge

    return gauge_collection[name]

def update_gauages_from_gmail(*unused_arguments_needed_for_scheduler):
    global GMAIL_CLIENT
    GMAIL_CLIENT = get_gmail_client()

    logging.info("Got gmail client successfully")

    set_readiness("")

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

    update_gauages_custom_message_queries()

def update_gauages_custom_message_queries():
    logging.info("Updating custom message queries - starting (%s)", str(len(args.customQueries)))

    for customQuery in args.customQueries:
        logging.info("Updating custom message queries: %s", customQuery['name'])

        try:
            search_result = GMAIL_CLIENT.users().messages().list(q=customQuery['query'], userId='me').execute()

            gauge = get_gauge_for_query(customQuery['name'])
            gauge.set(search_result['resultSizeEstimate'])
        except Exception as e:
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

def set_readiness(message):
    global READINESS
    READINESS = message

    logging.info("Readiness: %s", message)

def get_gmail_client():
    while not authComplete:
        logging.info("Waiting for credentials, sleeping for %d seconds", args.updateDelaySeconds)
        sleep(args.updateDelaySeconds)

    return discovery.build('gmail', 'v1', credentials = get_credentials())

def infinate_update_loop():
    while True:
        update_gauages_from_gmail()
        sleep(args.updateDelaySeconds)


def getFlow():
    flow = Flow.from_client_secrets_file(
        args.clientSecretFile,
        SCOPES,
        redirect_uri=args.oauthHost + '/oauth2callback'
    )

    flow.user_agent = 'prometheus-gmail-exporter'

    return flow

@flaskapp.route('/')
def index():
    ret = "<h1>prometheus-gmail-exporter</h1><br />"
    ret += "State: " + READINESS + "<br />"

    if not authComplete:
        flow = getFlow()

        authorization_url, state = flow.authorization_url()
        session['state'] = state

        ret += f'<a href="{authorization_url}">Login</a>'

    return ret

@flaskapp.route('/oauth2callback')
def oauth2callback():
    flow = getFlow()
    flow.fetch_token(authorization_response = request.url)

    state = session['state']

    if not request.args.get('state') == state:
        return 'Error: state mismatch', 400

    credentials = flow.credentials

    logging.info("Storing credentials to %s", args.credentialsPath)

    with open(args.credentialsPath, 'w', encoding='utf8') as token:
        token.write(credentials.to_json())

    set_readiness("GOT_CREDENTIALS")

    return f'Credentials: {credentials.token}'

@flaskapp.route('/readyz')
def readyz():
    if READINESS == "":
        return "OK"

    return Response(READINESS, status=200)

def start_waitress():
    logging.info("Starting on port %d", args.promPort)

    waitress.serve(flaskapp, host = '0.0.0.0', port = args.promPort)

def logVersion():
    if os.path.exists("VERSION"):
        with open("VERSION", 'r', encoding='utf8') as version_file:
            logging.info("Version: %s", version_file.read().strip())

def initArgs():
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
    parser.add_argument("--daemonize", "-d", action='store_true')
    parser.add_argument("--logLevel", type=int, default = 20)
    parser.add_argument("--customQueries", nargs='*', type=yaml.safe_load)

    global args
    args = parser.parse_args()

    logVersion()
    logging.getLogger().setLevel(args.logLevel)
    logging.info("args (from config, and flags): %s", args)
    logging.info("UID: %s", os.getuid())
    logging.info("Home directory: %s", os.getenv("HOME"))

def main():
    logging.getLogger().setLevel("INFO")
    logging.info("prometheus-gmail-exporter is starting up.")

    set_readiness("MAIN")
    initArgs()

    flaskapp.wsgi_app = DispatcherMiddleware(flaskapp.wsgi_app, {
        '/metrics': make_wsgi_app()
    })

    t = Thread(target = start_waitress)
    t.start()

    if args.daemonize:
        infinate_update_loop()
    else:
        update_gauages_from_gmail()

if __name__ == '__main__':
    try:
        main()
    except KeyboardInterrupt:
        print("\n") # Most terminals print a Ctrl+C message as well. Looks ugly with our log.
        logging.info("Ctrl+C, bye!")
