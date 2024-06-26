# prometheus-gmail-exporter

Checks gmail labels for unread messages and exposes the counts via prometheus.

There is a blog [article about why this was created, with an example integration to Grafana](https://medium.com/james-reads-public-cloud-technology-blog/watching-gmail-labels-with-prometheus-grafana-87b6745acd48). It looks like this;

![Grafana screenshot](doc/grafanaScreenshot.png)

## Example prometheus Metrics

```sh
# HELP gmail_INBOX_total INBOX Total
# TYPE gmail_INBOX_total gauge
gmail_INBOX_total 44351.0
# HELP gmail_INBOX_unread INBOX Unread
# TYPE gmail_INBOX_unread gauge
gmail_INBOX_unread 43.0
# HELP gmail_Label_33_total >/0. Triage Total
# TYPE gmail_Label_33_total gauge
gmail_Label_33_total 159.0
# HELP gmail_Label_33_unread >/0. Triage Unread
# TYPE gmail_Label_33_unread gauge
gmail_Label_33_unread 0.0
# HELP gmail_fooquery fooquery
# TYPE gmail_fooquery gauge
gmail_fooquery 201.0
```

## Example configuration file (`prometheus-gmail-exporter.yaml`)

```yaml
labels:
  - Label_33

customQueries:
  - name: fooquery
    query: "important in:inbox"
```

## Setup API access

To allow this app to access your gmail, it needs a `client_secret.json` file, and a `login_cookie.dat`. The instructions below explain how to set this up.

### Getting `client_secret.json`

**NOTE**: Google frequently changes it's API, Web interface, and similar without much notice. These instructions were last tested and validated on 2024-04-14 - if the instructions or screenshots don't match what you're seeing, it would be really helpful if you could raise a GitHub issue on this repository and report it. Thanks!

* Go to the Google Developers API Console: https://console.developers.google.com/apis/credentials
* In the sidebar, select **Credentials**. 
* On the **Credentials** page, click the **Create Credentials** button in the toolbar.

![Create credentials](doc/createCredentials.png)
 
* From the **Create Credentials** dropdown menu, select **OAuth Client ID**.
  * **Application Type:** _Desktop App_ (previously labeled _Other_, do not choose _Web application_).
  * **Name:** prometheus-gmail-exporter (or a name that you prefer)
  * Finally, click the blue **Create** button.
* On the **OAuth client created** popup window;
  * Take a copy of your **Client ID** and save it in a "secure text file" somewhere!
  * Click the "Download JSON" link to get your client_secrets file, named something like - `client_secret_1234.apps.googleusercontent.com__.json`.

### Create a OAuth 2 Content screen

* In the sidebar, select **OAuth 2 consent screen**
* Create a OAuth 2 Consent screen with whatever name and icon you like. The scopes needed are;

![Consent Screen](doc/consentScreenScopes.png)

* Once the scopes have been selected, you should see a screen that looks like this;

![Consent Screen after the scopes have been added](doc/consentScreenScopesAdded.png)

### Next steps

* Rename your downloaded `client_secret_____.json` to just `client_secret.json`
  and put it in the directory; `~/.prometheus-gmail-exporter/`.

**NOTE**: To use this on [Google Accounts with Advanced Protection](https://landing.google.com/advancedprotection/), you cannot use [an _OAuth consent screen_ that is only in _Publishing status = Testing_,](https://support.google.com/cloud/answer/10311615) but must _Publish the App_ and [then have it verified](https://support.google.com/cloud/answer/9110914).

### Getting `login_cookie.dat`

The `~/.prometheus-gmail-exporter/client_secret.json` file which you download from Google only identifies (your own instance of) this app to talk to the GMail API.

The `~/.prometheus-gmail-exporter/login_cookie.dat` secret file identifies YOU, and gives access to your Gmail account via Google's API. This file cannot be directly downloaded from the Google Cloud Console, but is created by this tool on its first run, via an OAuth-based flow. It will open a local web browser to a Google Login. If this fails (e.g. due to an _"Error 400: redirect_uri_mismatch"),_ then you can _visit an URL to authorize this application,_ which is printed by the tool on its first run. Both will (should) redirect to `http://localhost:9090/` (which has to be added as an _Authorized redirect URI_ to the _OAuth 2.0 Client ID)_ to _complete the authentication flow,_ which then creates this file. (With that, the tool will then proceed further, and on the next run possibly print a message with a URL to click on to enable the Gmail API.)

To run this tool on a headless server, you may want to first create the `login_cookie.dat` on a Desktop/Workstation where a web browser is a available, and then move it to the headless server, perhaps by mounting this file from some form of secret provider into the container. (See also [issue #9](https://github.com/jamesread/prometheus-gmail-exporter/issues/9) for more background.)

## Troubleshooting: `accessNotConfigured`

The GMail API is probably not enabled in your account yet. You can enable it by following instructions here; https://developers.google.com/workspace/guides/enable-apis . 

## Run as a container image

There is a published container in **hub.docker.com**, called `jamesread/prometheus-gmail-exporter:latest`.

Using either `docker` or `podman` will be fine. I like `podman` better, so
examples are with podman.

```
podman run ghcr.io/jamesread/prometheus-gmail-exporter:latest -v ~/.prometheus-gmail-exporter/:/root/.prometheus-gmail-exporter/
```

## Build your own container image

```
podman build . -t gmail-exporter
podman run -v ~/.prometheus-gmail-exporter/:/root/.prometheus-gmail-exporter/ gmail-exporter --labels Label_33
```

## Running via command line

### Option A) Python3 + PIP

```
user@host: pip install -r requirements.txt
user@host: ./gmail-exporter.py --labels Label_33 INBOX
```

Options can be found with `--help`.

### Option B) Fedora/Red Hat distributions

```
user@host: dnf install -y python3-configargparse python3-oauth2client python3-google-api-client python3-google-auth-oauthlib python3-prometheus_client
user@host: ./gmail-exporter.py --labels Label_33 INBOX
```

Options can be found with `--help`.
