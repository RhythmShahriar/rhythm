A message your site sends can still land in spam, or be rejected, because of the sending domain's DNS. This page covers `rhythm echo check`, which checks SPF, DKIM, DMARC, MX, MTA-STS, TLS-RPT and BIMI for the service you actually send through. It also covers `rhythm echo headers`, which reads a received email and says who sent it and why it passed or failed, with every issue each one reports.

```sh
rhythm echo check yoursite.com --provider sendgrid        # DNS for the service you send with
rhythm echo check yoursite.com --from-mail received.eml   # or learn the sender from a real email
rhythm echo headers received.eml --hops                   # what the receiving server concluded
```

> [!NOTE]
> Neither command sends anything. `check` makes DNS lookups, plus one HTTPS GET to the domain's own `mta-sts` host when you add `--mta-sts-fetch`. `headers` reads a file; with `--check` it also makes the DNS lookups. Both are safe on production.

## check

`check <domain>` looks up the domain's mail records and reports what's wrong with them. A generic check (is there an SPF record at all?) becomes a specific one when Echo knows the **sender**: does SPF authorize SendGrid, is SendGrid's `s1` key published, does mail from SendGrid align with DMARC.

| Flag | Tells Echo |
|---|---|
| `--provider <name>` | The sending service: `sendgrid`, `mailgun`, `postmark`, `ses`, `resend`, `brevo`, `mailchimp`, `mandrill`, `sparkpost`, `google`, `microsoft`, `php-mail`. |
| `--from-mail <file.eml>` | Learn the service, the DKIM selector and domain, the Return-Path domain and the relay from a message the site sent, received in a real inbox. |
| `--selector <s>` | The DKIM selector to check. |
| `--return-path <domain>` | The envelope sender's domain, which SPF checks. |
| `--smtp-host <host>` | The relay that hands mail out, a host name or IP, checked against SPF. |
| `--mta-sts-fetch` | Also fetch and check the MTA-STS policy file. |
| `--resolver <host:port>` | Ask this DNS server instead of the system's. |
| `--json` | The parsed records and the issues, as JSON. |

`check` takes a domain (`shop.com`); an address works too (`orders@shop.com` checks `shop.com`), a URL doesn't. The flags given override what `--from-mail` found. An unknown `--provider` name is a usage error that lists the known ones: `rhythm echo: --provider "bogus": use one of sendgrid, mailgun, postmark, ses, resend, brevo, mailchimp, mandrill, sparkpost, google, microsoft, php-mail`. `Sender:` shows the service's name (`SendGrid`, `Amazon SES`).

The output below comes from a fixture DNS server on `127.0.0.1` (`--resolver`), serving a zone for `shop.test` with two SPF records and a monitoring-only DMARC policy:

```sh
rhythm echo check shop.test --provider sendgrid --resolver 127.0.0.1:18753
```

```text
rhythm echo 1.1.1 - shop.test

Sender: SendGrid
Records: found DMARC, MX; no usable SPF, DKIM

[ERROR] Multiple SPF records - shop.test
  shop.test publishes 2 v=spf1 records; receivers treat that as a permanent error and SPF fails for all mail:
  fix: Merge them into one TXT record at shop.test (one v=spf1, every include, one all at the end) and delete the others.
[WARNING] No DKIM key found for the sending service - shop.test
  None of SendGrid's default selectors has a key: s1._domainkey.shop.test, s2._domainkey.shop.test. Without a DKIM signature from your own domain, DMARC can only pass on SPF, which breaks when mail is forwarded.
  fix: Set up domain authentication in SendGrid and publish the DKIM records it gives you under shop.test.
[WARNING] DMARC sends no aggregate reports - shop.test
  _dmarc.shop.test has no rua= tag, so nobody learns which servers send as shop.test or what fails.
  fix: Add rua=mailto:dmarc@shop.test (or a DMARC report service's address) to the TXT record at _dmarc.shop.test.
[NOTICE] DMARC policy is none - shop.test
  p=none is monitoring only: receivers send reports but do nothing to mail that fails DMARC, so spoofing shop.test still works. It meets the Gmail and Yahoo bulk-sender minimum. Move on once the reports show all legitimate mail passes.
[NOTICE] No MTA-STS policy - shop.test
  Without MTA-STS, servers sending to shop.test fall back to plain text if TLS is stripped in transit.

Delivery has errors - 1 error, 2 warnings, 2 notices
```

It exits `1`: without `--fail-on`, `check` fails on an error. The `Records:` line lists the records Echo could read and the ones it found missing or unusable (here two SPF records, so neither counts). Each issue ends with the domain it's about. After the fixes, the same zone with one SPF record including SendGrid, the `s1` key and `p=quarantine` with reports:

```text
rhythm echo 1.1.1 - shop.test

Sender: SendGrid
Records: found SPF, DKIM, DMARC, MX

[NOTICE] No MTA-STS policy - shop.test
  Without MTA-STS, servers sending to shop.test fall back to plain text if TLS is stripped in transit.
[NOTICE] No BIMI record - shop.test
  DMARC is enforced, so shop.test qualifies for BIMI: supporting inboxes can show its logo next to its mail.

All clear - 0 errors, 0 warnings, 2 notices
```

With a message the site sent, Echo also checks the Return-Path domain SendGrid uses for bounces:

```sh
rhythm echo check shop.test --from-mail received.eml
```

```text
[WARNING] Bounce domain has no SPF record - shop.test
  Mail is sent with Return-Path domain em123.shop.test. SPF checks that domain, which publishes no v=spf1 record.
  fix: Publish the TXT record your sending service gives for em123.shop.test (typically "v=spf1 include:sendgrid.net ~all").
```

`check` exits `1` on an error by default, like `run`; `--fail-on warning` or `notice` is stricter and `--fail-on none` never fails on what it finds.

### When the resolver doesn't answer

Each lookup has 3 seconds and the whole check 15. When no lookup gets any answer (the resolver refuses or never replies), `check` reports that once instead of a warning per record. It exits `3` whatever `--fail-on` says, since nothing was checked:

```sh
rhythm echo check shop.test --resolver 127.0.0.1:20463
```

```text
rhythm echo 1.1.1 - shop.test

rhythm echo: Could not reach the resolver 127.0.0.1:20463 (connection refused)
  Check that a DNS server is running at 127.0.0.1:20463 and answers queries from this machine, or leave out --resolver to use the system's.
```

Without `--resolver` it's `Could not reach the system resolver`, with the fix `Check this machine's DNS settings and network connection, or pass --resolver host:port to ask another DNS server.` With `--json`, the report has one `dns/resolver` error, `"unreachable": true`, the `"resolver"` asked, `"complete": false` and `"verdict": "error"`. A resolver that answers some lookups isn't unreachable: the failed records get their own warnings. `--resolver` without a port asks port 53.

### SPF

| Level | Issue | Fix, in short |
|---|---|---|
| error | No SPF record | Publish `v=spf1 … ~all` listing the services that send for the domain. |
| error | Multiple SPF records | Merge them into one record. |
| error | SPF allows every server (+all) | Change `+all` to `~all` (or `-all`). |
| error | SPF needs more than 10 DNS lookups | Remove unused includes, replace `a` and `mx` with `ip4:`/`ip6:`, or move a sender to its own subdomain. The detail counts every `include`, `a`, `mx`, `ptr`, `exists` and `redirect`, through every include and lists them. |
| error | SPF has more than 2 void lookups | Remove terms that point at names without records. |
| error | SPF does not authorize the sending service | Add the service's include (`include:sendgrid.net`), or the relay's `ip4:`, before the `all` term. Needs a known sender. |
| error | SPF record has a syntax error | Fix the listed terms. |
| error | SPF includes a domain without an SPF record | Remove or correct the include. |
| error | SPF includes a domain with multiple SPF records | Remove or replace the include. |
| error | SPF includes loop | Remove the include that points back up the chain. |
| warning | SPF is neutral for every other server (?all) | Change `?all` to `~all`. |
| warning | SPF record has no all mechanism | End it with `~all`. |
| warning | Bounce domain has no SPF record | Publish your service's record at the Return-Path domain. |
| warning | SPF lookup failed, SPF include lookup failed | Check the nameservers answer TXT queries. |
| notice | SPF uses the ptr mechanism | Replace `ptr` with `ip4:`/`ip6:` or an include. |

### DKIM

| Level | Issue | Fix, in short |
|---|---|---|
| error | DKIM key not found for the sending selector | Publish the record your service gives for `<selector>._domainkey.<domain>`. From `--selector` or a received message. |
| error | DKIM key is revoked | An empty `p=`: publish the current key, or switch selector. |
| error | DKIM record is invalid | Republish exactly as given: `v=DKIM1; k=rsa; p=<base64 key>`. |
| error | DKIM key is shorter than 1024 bits | Receivers ignore it (RFC 8301): generate a 2048-bit key. |
| warning | No DKIM key found for the sending service | None of the service's default selectors has a key: set up domain authentication. |
| warning | DKIM lookup failed | Check the nameservers. |
| notice | DKIM key is 1024 bits | Rotate to 2048 bits under a new selector. |
| notice | DKIM has only Ed25519 keys | Add an RSA key too; many receivers don't verify Ed25519 yet. |
| notice | DKIM selector unknown for the sending service | Postmark, SES and SparkPost generate selectors: run `headers` on a message, then check with its selector. |
| notice | No DKIM key found at common selectors | Without a known sender or selector Echo can only guess; give `--selector` or `--from-mail`. |

### DMARC

| Level | Issue | Fix, in short |
|---|---|---|
| error | No DMARC record | Publish `v=DMARC1; p=none; rua=mailto:dmarc@<domain>`, read the reports, then raise `p`. Neither the domain nor its organizational domain has one. |
| error | Multiple DMARC records | Keep one. |
| error | DMARC record is invalid | Republish it; an invalid record counts as none. |
| error | Mail from the sending service fails DMARC alignment | Neither the DKIM `d=` nor the Return-Path uses the From domain: set up domain authentication in the service. |
| error | DMARC strict alignment fails for the sending service | Sign and bounce at the exact domain, or set `adkim=r` and `aspf=r`. |
| warning | DMARC sends no aggregate reports | Add `rua=`. |
| warning | DMARC report address hasn't authorized reports | An outside `rua` domain must publish `v=DMARC1` at `<domain>._report._dmarc.<their domain>`. |
| warning | DMARC lookup failed | Check the nameservers. |
| notice | DMARC policy is none | Move to `quarantine`, then `reject`, once reports are clean. |
| notice | DMARC policy applies to only part of the mail | Raise `pct` to 100. |
| notice | DMARC leaves subdomains unprotected | Remove `sp=none`. |
| notice | DMARC passes on SPF alone | Have the service sign with your domain, since forwarding breaks SPF. |

Alignment is judged against the organizational domain (the registrable domain under the public suffix list), as receivers do.

### MX, MTA-STS, TLS-RPT and BIMI

| Level | Issue | Fix, in short |
|---|---|---|
| warning | No MX record | Replies and bounces can't be delivered. Some receivers reject mail from a domain that can't receive it. |
| warning | MX host does not resolve | Correct or remove the MX records listed. |
| notice or warning | Domain has a null MX | `MX 0 .` says the domain accepts no mail; a warning when a sender is known, since receivers may reject its mail (RFC 7505). |
| warning | MX lookup failed | Check the nameservers. |
| notice | No MTA-STS policy | Publish `_mta-sts.<domain>` and the policy file. |
| warning | Multiple MTA-STS records, MTA-STS record has no id | Keep one record, with an `id=` you change with the policy. |
| error | MTA-STS policy file cannot be fetched | With `--mta-sts-fetch`: serve `https://mta-sts.<domain>/.well-known/mta-sts.txt` with a valid certificate and no redirect. |
| error | MTA-STS policy file is invalid | It needs `version: STSv1`, `mode`, `max_age` and `mx` lines. |
| error | MTA-STS policy does not cover the MX hosts | Add `mx:` lines for the hosts listed. |
| notice | MTA-STS is in testing mode | Switch to `enforce` once TLS-RPT reports are clean. |
| notice | No TLS-RPT record | With MTA-STS set up, publish `_smtp._tls.<domain>` with a `rua=`. |
| warning | TLS-RPT record has no rua | Add `rua=mailto:…`. |
| notice | No BIMI record | With DMARC enforced, the domain qualifies for a logo in supporting inboxes. Optional. |
| notice | BIMI needs DMARC enforcement | `default._bimi` exists but DMARC isn't `quarantine` or `reject` at `pct=100`. |

## headers

`headers <file.eml>` reads a message as received, with all its headers: in Gmail, **Show original → Download original**; in Outlook, **View → View message source**. Pass `-` to read stdin. It reports the receiving server's verdict (from `Authentication-Results`, or the newest `ARC-Authentication-Results`), the DKIM signatures, the sending service and, with `--hops`, the route. These examples use the test messages in the Echo repository's `headers/testdata`:

```sh
rhythm echo headers phpmail.eml --hops
```

```text
rhythm echo 1.1.1 - phpmail.eml

From: wordpress@shop.example
Sent by: PHP mail() on the web server
Results: SPF softfail · DKIM none · DMARC fail
Hop 1:  → web01.shop.example 
Hop 2: web01.shop.example → mx.google.com ESMTPS
Hop 3:  → 2002:a05:6214:1c8e:b0:6b5:2e1d:3f10 SMTP

[ERROR] DMARC failed - phpmail.eml
  mx.google.com reports dmarc=fail for shop.example: neither SPF nor DKIM passed for a domain aligned with From. Mail like this goes to spam or is rejected under p=quarantine/reject.
  fix: Run `rhythm echo check shop.example` with this message's sender, then publish the DKIM records of the sending service under shop.example.
[ERROR] Message is not DKIM-signed - phpmail.eml
  There's no DKIM-Signature header. Without one DMARC can pass only on SPF, which breaks when mail is forwarded. Gmail and Yahoo require DKIM from bulk senders.
  fix: Send through a service that signs mail (or enable DKIM signing on the server) and publish its key under shop.example.
[WARNING] SPF softfail - phpmail.eml
  mx.google.com reports spf=softfail: the SPF record at web01.shop.example doesn't list the relay 203.0.113.7 and ends in ~all.
  fix: Add the sending service's include (or the server's ip4:) to the SPF record at web01.shop.example.
[WARNING] Mail header leaks a server script path - phpmail.eml
  X-PHP-Originating-Script: 33:class-phpmailer.php tells every recipient the server user ID and the script that sent the mail.
  fix: Set mail.add_x_header = Off in php.ini, or send through an SMTP or API mailer instead of mail().
[WARNING] Sent with PHP mail() from the web server - phpmail.eml
  The message was handed to the web server's local mail system, which doesn't sign it with DKIM and sends from an IP that mailbox providers distrust.
  fix: Send through an authenticated SMTP or API service (WP Mail SMTP, FluentSMTP or Laravel's mailers) with DKIM set up for shop.example.
[NOTICE] Mail header reveals the PHP version - phpmail.eml
  X-Mailer: PHP/8.1.2-1ubuntu2.18 shows the server runs PHP/8.1.2.

Delivery has errors - 2 errors, 3 warnings, 1 notice
```

It exits `1` on the errors. Each issue ends with the file it's about. A message from a properly set-up service is clean:

```text
rhythm echo 1.1.1 - postmark.eml

From: receipts@shop.example
Sent by: Postmark
Results: SPF pass · DKIM pass · DMARC pass

All clear - 0 errors, 0 warnings, 0 notices
```

Echo recognizes SendGrid, Mailgun, Postmark, Amazon SES, Resend, Brevo, Mailchimp, Mandrill, SparkPost, Google Workspace, Microsoft 365 and PHP `mail()` from the headers. `--check` runs the DNS checks above for the sender it found and takes `--resolver` and `--mta-sts-fetch` like `check` does. `--json` prints the whole analysis: From, Return-Path, the sender, the verdict per method, the signatures (`d=`, `s=`, algorithm, signed headers) and the hops.

| Level | Issue | Fix, in short |
|---|---|---|
| error | DMARC failed | Run `rhythm echo check` with this sender, then publish the service's DKIM records under the From domain. |
| error | SPF failed | Add the service's include, or the server's `ip4:`. |
| error | SPF record is broken (permerror) | Run `rhythm echo check` on the envelope sender's domain. |
| error | DKIM signature failed | The key is missing, or something changed the message after signing (a footer from a plugin or a mailing list). |
| error | Message is not DKIM-signed | Send through a service that signs mail. |
| error | DKIM signature does not cover From | Fix the signer's `h=` list. |
| error | From domain is not aligned with SPF or DKIM | Set up domain authentication in the service, so it signs with your domain. |
| error | From header is missing or invalid | Set a From address at your domain in the mailer settings. |
| warning | DMARC could not be evaluated, Receiver found no DMARC policy | Check, or publish, `_dmarc.<domain>`. |
| warning | SPF softfail, SPF found no record, SPF could not be evaluated | List the service in SPF; check the nameservers. |
| warning | DKIM signature uses SHA-1 | Switch the signer to `rsa-sha256`. |
| warning | DKIM is not aligned with the From domain | DMARC passes only through the Return-Path, which forwarding breaks. |
| warning | Message has no Message-ID, Message has no Date | Send through a mailer that sets them. |
| warning | Mail header leaks a server script path | `mail.add_x_header = Off`, or use an SMTP or API mailer. |
| warning | Sent with PHP mail() from the web server | Use an authenticated SMTP or API service with DKIM. |
| notice | No Authentication-Results header | The file has no receiver verdict: save the message as received, not as sent. |
| notice | SPF result is neutral | List the service and end SPF with `~all`. |
| notice | DKIM signature covers only part of the body | Turn off body-length limits (`l=`). |
| notice | Mail header reveals the PHP version | Remove or override `X-Mailer`. |
| notice | Bulk mail has no List-Unsubscribe header | Gmail and Yahoo require one-click unsubscribe from bulk senders. |
| notice | List-Unsubscribe has no one-click option | Add `List-Unsubscribe-Post: List-Unsubscribe=One-Click`. |

## What's never sent

- No test email, ever: Echo can't see how a message is received, so it reads what a receiver already concluded (`headers`) and the DNS that decides it (`check`).
- No request to the webhook endpoints, CRMs or automation hooks in your captures: Zapier, Make and n8n trigger on any request.
- No request to the mail servers in your MX records.
- The only HTTPS request is the optional `--mta-sts-fetch`, to `https://mta-sts.<domain>/.well-known/mta-sts.txt` on the domain you named, without following redirects.

---
<sub>← [Replay](Echo-Replay) · [Home](Home) · [Cookie consent](Echo-Consent) →</sub>
