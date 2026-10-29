Echo ships as a container image, `ghcr.io/rhythmshahriar/echo`, that drops in where Mailpit runs today: the inbox and API on port 8025, the SMTP sink on 1025, Mailpit's API for the tools that read it. This page covers running the image, Echo in Docker Compose, the DDEV add-on, Laravel Sail and Lando, what Echo answers to without a token and what to do when Mailpit already has the ports on your machine.

```sh
docker run -p 127.0.0.1:8025:8025 -p 127.0.0.1:1025:1025 ghcr.io/rhythmshahriar/echo:1
```

## The image

| | |
|---|---|
| Image | `ghcr.io/rhythmshahriar/echo`, for `linux/amd64` and `linux/arm64` |
| Tags | each release's version (`1.1.1`), its major version (`1`) and `latest`; a prerelease gets only its own tag |
| Base | `gcr.io/distroless/static:nonroot`, pinned by digest: no shell, no package manager |
| User | `nonroot` (uid 65532) |
| Ports | `8025` (API and Echo Inbox), `1025` (SMTP) |
| Entrypoint | `rhythm-echo up` |
| Default flags | `--listen=0.0.0.0:8025 --smtp=0.0.0.0:1025 --private-network` |
| State | `/home/nonroot/.config/rhythm/echo`: the proxy's CA and its key. Mount a volume there to keep them. |

The image holds the same release binary as the other downloads; the release job checks the binaries' signatures before it builds the image, then signs the image by digest with cosign (keyless). Check it:

```sh
cosign verify ghcr.io/rhythmshahriar/echo:1 \
  --certificate-identity-regexp '^https://github\.com/RhythmShahriar/' \
  --certificate-github-workflow-repository RhythmShahriar/echo \
  --certificate-oidc-issuer https://token.actions.githubusercontent.com
```

Arguments to `docker run` replace the default flags: `docker run … ghcr.io/rhythmshahriar/echo:1 --observe --listen=0.0.0.0:8025`. Settings come from flags and the `RHYTHM_ECHO_*` variables: `RHYTHM_ECHO_TOKEN`, `RHYTHM_ECHO_HOSTNAMES`, `RHYTHM_ECHO_LOGIN` and the rest in [Configuration](Echo-Configuration#environment-variables).

## Without a token, on a private network

Echo refuses to listen beyond loopback without a token, since anyone who reaches the API can read what the site sent. A container has to listen on `0.0.0.0`, so the image starts Echo with `--private-network`: it may listen without a token when every address it can be reached on is loopback, a private range (RFC 1918, `fc00::/7`), the shared range `100.64.0.0/10` or a container interface with such an address. The banner warns:

```text
rhythm echo 1.1.1 - this machine

Mode: seal: nothing leaves; Echo answers for every service
API: http://0.0.0.0:8025  (connectors post here; open it for the live view)
SMTP: 0.0.0.0:1025  (point a mailer or sendmail_path here)
Warning: no token on 0.0.0.0:8025, 0.0.0.0:1025: anything on this private network can post to Echo and read what it caught (--private-network)
```

This is the image's default command, run with the release binary in a container on an internal Docker network, as user 65532; another container on that network reached it by its service name, `echo`:

```sh
printf 'To: ann@shop.test\nSubject: Order confirmed\n\nThanks.\n' | RHYTHM_ECHO_SMTP=echo:1025 rhythm-echo sendmail -t -i
RHYTHM_ECHO_URL=http://echo:8025 rhythm-echo ls
```

```text
86cdda2e2623  14:39:52 email to ann@shop.test: Order confirmed [faked]
```

On a host with a public address, `--private-network` refuses (exit `2`):

```text
rhythm echo: --listen 0.0.0.0:8025 is reachable from a public address (203.0.113.5 on eth0); --private-network allows only private (RFC 1918) and container networks: listen on a private address, or set --token (or RHYTHM_ECHO_TOKEN)
```

### What Echo answers to

A web page can make your browser send requests to `localhost` or to a name of its own that points at a private address (DNS rebinding). So, without a token, Echo checks the `Host` header and answers only to:

- loopback names: `localhost`, `*.localhost`, `127.0.0.1`, `[::1]`;
- with `--private-network`, from a peer on loopback or a private network: IP addresses (`172.18.0.2:8025`) and names of one label (`echo:8025`, `mailpit`), which a container uses for a service;
- the names `--hostname` (repeatable) or `RHYTHM_ECHO_HOSTNAMES` (comma-separated) give, such as `--hostname '*.ddev.site'`, which takes `shop.ddev.site` but not `ddev.site`.

Anything else gets `403`:

```text
Echo answers only to localhost: Host rebind.attacker.example is not allowed
```

A request carrying a web page's `Origin` is refused too (`Echo refuses requests from web pages: Origin https://evil.example is not allowed`), as is a body that isn't JSON from a browser (`send JSON: Content-Type must be application/json`). `--hostname` takes a name without a port: `rhythm echo: --hostname "echo:8025": want a host name like echo.myapp.lndo.site or *.ddev.site, without a port` (exit `2`).

### With a token

Set `RHYTHM_ECHO_TOKEN` to reach Echo from anywhere else, or to keep other containers on the network out. The API then wants it in `X-Echo-Token` and the SMTP sink as the password, with any username. Beyond loopback a token needs 16 characters or more:

```text
rhythm echo: --token (or RHYTHM_ECHO_TOKEN): shorter than 16 characters, which can be guessed while 0.0.0.0:18025 is reachable from other machines; make one with: openssl rand -hex 16
```

A [viewer login](Echo-Inbox#a-viewer-login-for-clients) on a private network needs the token too: connectors use the token, viewers the login.

## Docker Compose

As a service next to the site, in place of Mailpit:

```yaml
services:
  echo:
    image: ghcr.io/rhythmshahriar/echo:1
    ports: ["127.0.0.1:8025:8025"]            # Echo Inbox on the host only
    volumes: ["echo-state:/home/nonroot/.config/rhythm/echo"]
  app:
    environment:
      RHYTHM_ECHO_URL: http://echo:8025        # for the connectors
      RHYTHM_ECHO_SMTP: echo:1025              # for rhythm-echo sendmail
      MAIL_HOST: echo                          # for an SMTP mailer
      MAIL_PORT: "1025"
volumes:
  echo-state: {}
```

Bind published ports to `127.0.0.1`: without a token anything that reaches them can read the inbox. To seal the app's network so nothing gets out except through Echo, use the hard seal recipe in [Sealing staging](Echo-Sealing-Staging#docker-internal-network), which runs this image with its proxy and a token. A Compose file from before 1.1 mounted the state at `/home/nonroot/.cache/rhythm/echo`: move the mount, or Echo moves the CA out of the volume on its next start and makes a new one each time the container is recreated.

## DDEV

The add-on replaces DDEV's built-in Mailpit with Echo, with no change to the app. It needs DDEV v1.24.10 or later.

```sh
ddev add-on get https://github.com/RhythmShahriar/echo/releases/latest/download/ddev-echo.tar.gz
ddev restart
ddev echo        # opens the inbox: https://<project>.ddev.site:8026
```

`ddev-echo.tar.gz` is a release asset with a line in the release's `checksums.txt`; pin a release with `.../releases/download/v1.1.1/ddev-echo.tar.gz`.

| | Before | With Echo |
|---|---|---|
| Inbox on `https://<project>.ddev.site:8026` (and `http://…:8025`) | Mailpit | Echo |
| Mailpit's own inbox | 8026 | 8030 (`http://…:8029`) |
| `mail()`, SMTP to `localhost:1025` | Mailpit | Mailpit, relayed to Echo |
| Inside the project network | | `http://echo:8025` (API), `echo:1025` (SMTP) |

The add-on writes `docker-compose.echo.yaml` (the `echo` service, with `RHYTHM_ECHO_HOSTNAMES=$DDEV_HOSTNAME` so the router's name works), `config.echo.yaml` (Mailpit on ports 8029 and 8030, relaying everything to `echo:1025`) and `commands/host/echo` (`ddev echo`). For the connectors, add `RHYTHM_ECHO_URL=http://echo:8025` to `web_environment`. Another image, or a token:

```sh
ddev dotenv set .ddev/.env.echo --echo-docker-image=ghcr.io/rhythmshahriar/echo:1.1.1
token=$(openssl rand -hex 16)
ddev config --web-environment-add=RHYTHM_ECHO_URL=http://echo:8025,RHYTHM_ECHO_TOKEN=$token
ddev dotenv set .ddev/.env.echo --rhythm-echo-token=$token
ddev restart
```

With a token, Mailpit's relay can't log in to Echo (Mailpit won't send a password to another host without TLS), so `mail()` and SMTP to `localhost:1025` stop at Mailpit. Point the app's mailer at `echo:1025` with any username and the token as the password (Laravel: `MAIL_HOST=echo`, `MAIL_PORT=1025`, `MAIL_USERNAME=echo`, `MAIL_PASSWORD` set to the token), or leave the token unset on DDEV's private network. Remove it with `ddev add-on remove echo`, then `ddev restart`.

## Laravel Sail

Sail runs Mailpit as a service named `mailpit`. Keep the name and swap the image, so nothing in `.env` changes:

```yaml
    mailpit:
        image: 'ghcr.io/rhythmshahriar/echo:1'   # was axllent/mailpit:latest
        ports:
            - '127.0.0.1:${FORWARD_MAILPIT_PORT:-1025}:1025'
            - '127.0.0.1:${FORWARD_MAILPIT_DASHBOARD_PORT:-8025}:8025'
        networks:
            - sail
```

Then `sail up -d`: the inbox is at http://localhost:8025. To capture more than mail, add the [Laravel connector](Echo-Install-Connectors#laravel) with `RHYTHM_ECHO_URL=http://mailpit:8025`. With a token, set `RHYTHM_ECHO_TOKEN` on the service and `MAIL_USERNAME=echo`, `MAIL_PASSWORD=${RHYTHM_ECHO_TOKEN}` in `.env`.

## Lando

Lando replaces a service's entrypoint with a shell script and the image has no shell, so name Echo's binary as the entrypoint:

```yaml
services:
  echo:
    api: 3
    type: lando
    app_mount: false
    entrypoint: /usr/local/bin/rhythm-echo
    services:
      image: ghcr.io/rhythmshahriar/echo:1
      command: up --listen=0.0.0.0:8025 --smtp=0.0.0.0:1025 --private-network --hostname=echo.myapp.lndo.site
      ports:
        - 8025
  appserver:
    overrides:
      environment:
        MAIL_MAILER: smtp
        MAIL_HOST: echo
        MAIL_PORT: 1025
        RHYTHM_ECHO_URL: http://echo:8025

proxy:
  echo:
    - echo.myapp.lndo.site:8025
```

Then `lando rebuild -y` and open https://echo.myapp.lndo.site (your app's name there and in `--hostname`). This recipe isn't run in CI; the DDEV add-on is.

## Mailpit on the same machine

When Mailpit (from DDEV, Lando or Sail) already holds 8025 or 1025, `up` names it and offers the next free pair, from 18025 and 11025 upward:

```text
rhythm echo: Mailpit is already on 127.0.0.1:8025; run Echo next to it: rhythm echo up --listen 127.0.0.1:18025 --smtp 127.0.0.1:11025
```

That's piped or in CI (exit `3`). On a terminal it asks *Mailpit is already on 127.0.0.1:8025. Run Echo on 127.0.0.1:18025 and 127.0.0.1:11025 instead and save them to .echo/config.yaml?* `--free-ports` takes the pair without asking:

```sh
rhythm echo up --free-ports
```

```text
Mailpit is already on 127.0.0.1:8025. Echo runs on 127.0.0.1:18025 and 127.0.0.1:11025 instead.
Saved to .echo/config.yaml, so the next up uses them too.

Point the site at Echo:
  RHYTHM_ECHO_URL=http://127.0.0.1:18025
  RHYTHM_ECHO_SMTP=127.0.0.1:11025
  MAIL_MAILER=smtp
  MAIL_HOST=127.0.0.1
  MAIL_PORT=11025   (Laravel .env)
```

That was a Laravel project (it has an `artisan` file). For WordPress, plain PHP or an unknown project the third line is a `sendmail_path` for `php.ini`; Next.js and Node get only the two `RHYTHM_ECHO` lines. Only `listen:` and `smtp:` are written into the config; its other lines and comments stay as they were. MailHog and MailDev are named the same way. `doctor` says so too: `FAIL  Echo: http://127.0.0.1:8025 is Mailpit, not Echo; start Echo on a free port (rhythm echo up --listen 127.0.0.1:18025) and set listen: in .echo/config.yaml to it` (exit `3`). When it's Echo itself: `rhythm echo: Echo is already running on 127.0.0.1:8025 (use that one, or stop it first)`.

---
<sub>← [Evidence](Echo-Consent-Evidence) · [Home](Home) · [Agency hub and reports](Echo-Hub-And-Reports) →</sub>
