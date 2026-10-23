Rhythm's tools exist to help you find and fix weaknesses in **your own** websites before someone else does. This page sets out who may use them and on what, what Tuner does and never does, and how to get permission when you test a site for a client.

> [!IMPORTANT]
> Scan only sites you own, or sites you have explicit written permission to test. Even a gentle, read-only scan of someone else's site without permission may be illegal.

## Authorized use only

Use Rhythm tools only on:

- systems you own and control, or
- systems whose owner has given you **explicit written permission** to test them, within the scope and time they agreed.

Scanning systems without authorization may be illegal where you are or where the target is, for example under the U.S. Computer Fraud and Abuse Act, the UK Computer Misuse Act 1990, or similar laws elsewhere. Hosting providers, CDNs and web application firewalls may also have their own terms that limit testing.

Owning a domain doesn't always mean you may scan the server behind it. If your site runs on shared hosting or a managed platform, check the provider's terms first.

## Your responsibility

By using Rhythm tools you agree to comply with all applicable laws and the terms of any service you test. You are responsible for how you use them. The authors and contributors are not responsible for misuse, or for any damage, legal consequences or loss arising from their use.

## No warranty

Rhythm and its tools are free software under the [GNU Affero General Public License v3.0](https://github.com/RhythmShahriar/rhythm/blob/master/LICENSE). They are provided "as is", without warranty of any kind, and the authors' liability is limited, as set out in **section 15 (Disclaimer of Warranty)** and **section 16 (Limitation of Liability)** of the license.

Findings can include false positives and can miss real issues. Verify a finding before you act on it, and don't treat a clean scan as proof a site is secure. Read [Reading Results](Tuner-Reading-Results) to see how Tuner marks findings it's less sure of.

## What Tuner does

Tuner detects. It never exploits.

- **Remote scans** send ordinary HTTP **GET** requests for public URLs, the same requests a browser or search engine crawler could send, and look at the replies. It also opens a TLS connection to see which protocol versions and certificate the server offers.
- **Rate limited by default:** at most 5 requests per second per site, 10 in flight. It slows down when the server answers `429` or `503`, honors `Retry-After`, and identifies itself with an honest user agent. See [Requests](Tuner-Requests).
- **Secrets are always masked.** When Tuner finds a credential, such as a database password in an exposed `.env`, it reports that it found one and masks the value in every output format.
- **Version matching happens on your machine**, against public vulnerability data. Tuner doesn't try a vulnerability to see if it works.
- **Local scans** of a directory only read files. They never change them and never run the site's code; the one command Tuner runs is the `php` binary, to ask for its version. See [Local Scans](Tuner-Local-Scans).

## What Tuner never does

- Send exploit payloads, injection strings or proof-of-concept attacks.
- Log in, submit forms, or send `POST`, `PUT` or `DELETE` requests to the site. (You can pass your own headers or cookies, for example to reach a staging site behind basic auth; Tuner only sends them with its GET requests.)
- Try passwords or brute-force anything.
- Change, upload or delete anything on the site.
- Print a secret it finds in full.

Checks are declarative YAML files that can only describe GET requests and how to match the reply, so a check, including one delivered in a [check pack](Tuner-Check-Packs), can't add any of these behaviors.

## The one-time notice

The first time you run a scan on a machine, Tuner prints this on stderr:

```text
Note: scan only sites you own or are authorised to test.
Tuner is non-intrusive and never sends exploit payloads. This notice is shown once.
```

The interactive picker (`rhythm tuner` with no arguments) shows the same reminder: "Only scan sites you own or are authorised to test." Seeing the notice once doesn't change the rules; they apply to every scan.

## Testing a site for a client

If you're an agency, freelancer or consultant, get written permission before you scan a client's site, even one you built or host. A short email or signed note from someone who can authorize it is enough for most engagements. It should say:

- **Who** is authorizing it, and that they have the authority to (the site owner, or someone acting for them).
- **Who** will run the tests: you or your company.
- **What** is in scope: the exact hostnames, for example `yoursite.com` and `staging.yoursite.com`, and anything out of scope.
- **What** you'll do: an automated, non-intrusive vulnerability scan with Tuner, read-only GET requests, rate limited, no exploitation. Mention a local scan if you'll run one on their server.
- **When**: the dates or window.
- **Who to contact** if the scan causes any trouble, on both sides.

Also check with the hosting provider or anyone else who runs part of the stack, such as a managed WordPress host or CDN, if their terms require notice.

Keep the permission with your report. For sites behind a firewall, ask the client to allowlist your IP or user agent rather than working around their protection, and use `--rate` to go gentler on fragile hosts.

## If you find a problem on someone else's site

If you come across a weakness on a site you weren't asked to test, don't scan it to confirm. Report what you noticed to the owner through their published security contact (often a `security.txt` file at `/.well-known/security.txt`) and let them investigate.

Found a vulnerability in Rhythm or Tuner itself? See the [Security Policy](Security-Policy).

---
<sub>← [FAQ and troubleshooting](Tuner-FAQ) · [Home](Home) · [Security policy](Security-Policy) →</sub>
