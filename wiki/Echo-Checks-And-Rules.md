Every check Echo runs on a message has an ID, a level and the channels it applies to; `rhythm echo checks` lists them all. This page covers that registry, how `checks:` in `.echo/config.yaml` turns a check off or changes its level, how `rules:` adds checks of your own (an old phone number, a rival's name, a retired brand) and how `checks --test` proves each rule with fixtures.

```sh
rhythm echo checks          # the checks, with your rules after them
rhythm echo checks --test   # try each rule on its fixtures
```

The checks themselves, with every title and fix, are in [Messages and checks](Echo-Messages-And-Checks#checks).

## The registry

```sh
rhythm echo checks
```

```text
CHECK                LEVEL    CHANNELS                                               FINDS
send/failed          error    all                                                    The site tried to send the message and the send failed.
content/placeholder  error    email,sms,whatsapp,chat                                A merge tag or placeholder ({first_name}, {{ name }}, *|FNAME|*) was never filled in.
content/php-error    error    email,sms,whatsapp,chat                                A PHP error, warning or notice was printed into the message.
content/encoding     warning  email,sms,whatsapp,chat                                Garbled characters (â€™, Ã©) from text that was encoded twice.
content/dev-link     warning  email,sms,whatsapp,chat                                A link points at a development address (localhost, *.test, a private IP) while the site isn't one.
email/no-subject     warning  email                                                  An email has no subject.
email/empty          error    email                                                  An email has neither text nor HTML.
email/no-text        notice   email                                                  An HTML email has no plain-text part.
email/clipped        warning  email                                                  An email's HTML is over 102 KB, so Gmail clips it.
email/from-domain    notice   email                                                  An email is sent from another registrable domain than the site's.
sms/number           warning  sms,whatsapp                                           An SMS or WhatsApp message goes to a number that isn't in E.164 form.
sms/parts            notice   sms                                                    An SMS needs more than one part (a warning over three); each part is charged.
webhook/json         warning  webhook                                                A webhook says it sends JSON but its body doesn't parse.
webhook/signature    warning  webhook                                                A WooCommerce webhook is sent without a signature.
content/secret       warning  sms,whatsapp,chat,webhook,analytics,payment,push,http  A payload carries a credential (Echo masked it).
seal/real-recipient  warning  all                                                    A message really went out (observe mode) to an address or number outside allow_recipients.
compat/html          notice   email                                                  An email's HTML uses a feature some common email clients drop (caniemail.com).
compat/spam          notice   email                                                  What SpamAssassin scores an email, when up runs with --spamassassin.
```

These run on every message an action sends and, since 1.1, on every capture: the live log of `up`, `watch`, Echo Inbox and the API show the same findings as a run. Comparisons add their own checks (`message/missing`, `message/text`, `message/attachment-lost` and the others in [Reading a comparison](Echo-Actions-And-Compare#the-classes)); `expect:` adds `expect/unmet`.

## Tuning a check

`checks:` sets a check's level, or turns it off, by its ID:

```yaml
# .echo/config.yaml
checks:
  content/dev-link: off          # links to staging are fine on this project
  sms/parts: warning             # every extra SMS part costs money
  message/attachment-changed: off   # the invoice PDF is made fresh on every run
```

| Value | In a run, a capture, `watch` and Echo Inbox | In a comparison |
|---|---|---|
| `off` | The check's findings are dropped. | Its changes are dropped, FIXED ones too. |
| `notice`, `warning`, `error` | The findings get that level. | `error` makes the change BROKE; `warning` and `notice` make it NEW. |
| `broke` | The findings become errors. | The change is BROKE. |
| `changed` | The level stays. | The change is CHANGED. |

A FIXED change stays FIXED. The overrides apply to comparisons against snapshots taken before you set them, so turning a check off hides it right away. `rhythm echo checks` ends with a line saying how many checks the config tunes, such as `.echo/config.yaml sets the level of 3 checks under checks:.` An ID no check has is refused when the config loads (exit `2`): `rhythm echo: /tmp/t1/.echo/config.yaml: line 2: checks: content/devlink isn't a check; rhythm echo checks lists them`.

To leave a field, a header or a recipient out of one action's comparison instead of switching a check off everywhere, use the action's `ignore:`; see [Actions and compare](Echo-Actions-And-Compare#ignore).

## Rules of your own

A rule is a check you write: a text or a regular expression that should never appear in what the site sends.

```yaml
# .echo/config.yaml
rules:
  - id: brand/old-phone
    message: Old phone number
    match: "0800 123 456"
    channels: [email, sms]
    level: error
    fixtures:
      match: ["Call us on 0800 123 456"]
      no_match: ["Call us on 0800 123 457"]
  - id: brand/old-name
    message: Old shop name
    regex: "(?i)bean (and|&) leaf ltd"
    fixtures:
      match: ["Bean & Leaf Ltd, 1 High Street"]
```

| Key | Required | Means |
|---|---|---|
| `id` | yes | The rule's ID, without spaces, such as `brand/old-phone`. It can't take a built-in check's ID or start with `message/`, `dns/`, `headers/`, `seal/`, `expect/` or `run/`. |
| `match` | one of the two | Text to look for, as written (case matters). |
| `regex` | one of the two | A Go (RE2) regular expression; `(?i)` ignores case. An empty match counts as no match. |
| `level` | no | `notice`, `warning` (the default) or `error`. |
| `channels` | no | The channels it looks at; all of them when left out. |
| `message` | no | The finding's title; `Matched <id>` when left out. |
| `fixtures` | no | `match:` texts the rule must find and `no_match:` texts it must leave alone. |

A rule looks at a message's subject, its visible text, then its body (the raw MIME for email) and reports once per message. Its findings are listed with the built-in checks, in every output, with the text it found masked like everything else Echo stores, so a rule written to catch a leaked key never copies the key into a report:

```text
[ERROR] Old phone number in email - Invoice
  found "0800 123 456"
  fix: Change the message, or the rule brand/old-phone in .echo/config.yaml if the text is fine now.
```

`checks` lists your rules after the built-in ones, their description ending in `(custom rule)`:

```text
brand/old-phone      error    email,sms                                              Old phone number (custom rule)
brand/old-name       warning  all                                                    Old shop name (custom rule)
```

`checks:` tunes a rule like any other check. A rule that doesn't load stops every command before it runs (exit `2`), with the line it's on:

```text
rhythm echo: /tmp/t1/.echo/config.yaml: line 2: rules[0]: needs an id without spaces, like brand/old-phone
rhythm echo: /tmp/t1/.echo/config.yaml: line 2: rule "a": set exactly one of match or regex
rhythm echo: /tmp/t1/.echo/config.yaml: line 4: rule "a": level "broke" isn't notice, warning or error
rhythm echo: /tmp/t1/.echo/config.yaml: line 4: rule "a" is listed twice
rhythm echo: /tmp/t1/.echo/config.yaml: line 2: rule "send/failed": that is a built-in check; give the rule its own id
```

## Testing rules

Every rule should have one fixture it matches and one it doesn't, the same as Echo's built-in checks. `checks --test` tries them:

```sh
rhythm echo checks --test
```

```text
ok    brand/old-phone matches "Call us on 0800 123 456"
ok    brand/old-phone leaves "Call us on 0800 123 457" alone
ok    brand/old-name matches "Bean & Leaf Ltd, 1 High Street"
WARN  brand/old-name: has no no_match fixture; give each rule one that matches and one that doesn't
2 rules, 3 fixtures: all pass, 1 missing.
```

A missing fixture is a warning, not a failure (exit `0`). A fixture that fails exits `1`:

```text
ok    brand/old-phone matches "Call us on 0800 123 456"
FAIL  brand/old-phone shouldn't match "Call us on 0800 123 456 ext 2" but does
ok    brand/old-name matches "Bean & Leaf Ltd, 1 High Street"
WARN  brand/old-name: has no no_match fixture; give each rule one that matches and one that doesn't
2 rules, 3 fixtures: 1 failed, 1 missing.
```

Until it passes, every other command refuses the config (exit `2`), so a rule that would let a mistake through never runs:

```text
rhythm echo: /home/you/projects/rules/.echo/config.yaml: line 14: rule "brand/old-phone": fixtures: matches "Call us on 0800 123 456 ext 2", which it shouldn't
```

Plain `checks` still lists everything, then prints that line with `  rhythm echo checks --test shows every fixture; other commands refuse the config until it passes` and exits `1`. With no rules: `No custom rules in .echo/config.yaml (add them under rules:).`

## HTML in email clients

`compat/html` reads an email's HTML and CSS for features that common clients drop: Gmail, Outlook for Windows, Outlook.com, Yahoo Mail, Apple Mail and Samsung Email. Its table is a hand-picked part of [caniemail.com](https://www.caniemail.com)'s data (MIT licensed), 44 features from `display: flex` and `grid` to `<style>` blocks over 16 KB, SVG images and web fonts. Only a feature some client drops outright is reported; one that's partly supported (a `<style>` block in Gmail) isn't.

```text
[NOTICE] Email uses display: grid, which Outlook for Windows, Gmail and Outlook.com drop - Newsletter
  display: grid (line 2): not in Outlook for Windows, Gmail, Outlook.com; partly in Yahoo Mail
```

Content inside `<!--[if mso]> … <![endif]-->` isn't checked. A feature used only in a `<!--[if !mso]>` block isn't held against Outlook for Windows. A table layout with inline styles gives no findings. Turn it off with `checks: {compat/html: off}`.

## SpamAssassin scores

With a [SpamAssassin](https://spamassassin.apache.org) daemon (`spamd`) running, `up --spamassassin host[:port]` (port 783 when left out) scores every email in the background (never holding up the site) and keeps the verdict on the message as the header `X-Echo-Spamassassin`: `score=2.1; threshold=5.0; spam=no; rules=HTML_MESSAGE,MISSING_DATE`. The `compat/spam` notice reports it: `SpamAssassin scores the email 2.1 of 5.0`, or `SpamAssassin calls the email spam (15.2 of 5.0)`, with the rules that hit. The banner says it's on:

```text
Spam: each email is scored by SpamAssassin at 127.0.0.1:20783
```

When spamd doesn't answer, the notice says so instead (here nothing listened on that port):

```json
{"check":"compat/spam","level":"notice","title":"SpamAssassin couldn't score the email","detail":"spamassassin at 127.0.0.1:20783: couldn't connect: connect: connection refused","fix":"Check that spamd runs and answers at the --spamassassin address."}
```

Echo speaks spamd's protocol itself, so there's no `spamc` to install. It only sends messages up to 500 KB, as `spamc` does. The score is SpamAssassin's opinion of the content; what decides delivery is mostly the sending domain's DNS, which [Deliverability](Echo-Deliverability) checks.

---
<sub>← [Messages and checks](Echo-Messages-And-Checks) · [Home](Home) · [Attachments](Echo-Attachments) →</sub>
