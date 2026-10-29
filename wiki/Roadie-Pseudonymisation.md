Pseudonymising replaces each real person's details with fakes that keep the shape of the data: an email stays an email, a phone stays a phone, a serialized value stays valid. The same person gets the same fake in every table and every encoding, so joins, lookups and order histories still work. This page covers what the fakes look like, how they stay consistent, what happens to passwords and free text and what pseudonymised does not mean.

> [!IMPORTANT]
> Pseudonymised data is still personal data under the GDPR (Article 4(5), Recital 26). The copy keeps order totals, dates, products, the structure of each person's history and anything Roadie can't recognise in free text. Whoever holds the consistency key can link fakes back to people. Protect the copy as you protect production.

## One person, one fake

The rows below are from the shop in [Getting started](Roadie-Getting-Started), before and after one copy. Jonas's billing address in `wp_wc_order_addresses`:

```text
(1,100,'billing','Jonas','Jablonski','','40 Beckett Street','','Portland','OR','97214','US','jonas.jablonski45@gmail.com','+1 503 237 6852')
(1,100,'billing','Fergus','Hunter','','621 Vale Crescent','','Woking','OR','99028','US','ue74fd9c452af3760@example.invalid','+1 555 012 6376')
```

An order note in `wp_comments` that mentions him, scrubbed in place:

```text
Customer rang from +1 503 237 6852: please send the invoice to jonas.jablonski45@gmail.com and leave the parcel next door.
Customer rang from +1 555 012 6376: please send the invoice to ue74fd9c452af3760@example.invalid and leave the parcel next door.
```

And his review reminder, queued in Action Scheduler as JSON:

```text
{\"email\":\"jonas.jablonski45@gmail.com\",\"order_id\":100}
{"email":"ue74fd9c452af3760@example.invalid","order_id":100}
```

The same email became the same fake in all three places. So did the phone. State and country stayed (`keep: [country, state]`, for tax and shipping rules); the postcode kept its shape. A PostgreSQL app copied with `columns:` rules behaves the same way, here `User`, `Session` (a JSON column) and `Address`:

```text
usr_000001	purufo.lobiva.d77n94@gmail.com	Rahageg Viherozih	+49 151 26665561	2026-05-01 10:00:00
usr_000001	u841cc5d3d9ede804@example.invalid	Gareth Newman	+1 555 019 6351	2026-05-01 10:00:00

ses_000001	usr_000001	185.193.178.200	{"email": "purufo.lobiva.d77n94@gmail.com", "cart": []}
ses_000001	usr_000001	198.51.100.40	{"email": "u841cc5d3d9ede804@example.invalid", "cart": []}

adr_000001	usr_000001	540 Regepukol Close	Dublin	IE
adr_000001	usr_000001	933 Chalk Street	Aberdeen	IE
```

## What the fakes look like

| Kind | Fake | Notes |
|---|---|---|
| email | `u<16 hex>@example.invalid` | The domain is `pseudonymise.email_domain` and must be reserved (`example.com`, `example.net`, `example.org`, or a name under `.example`, `.invalid`, `.test`, `.localhost`), so a fake can never receive mail. Two different emails with one fake stop the run (`fake-collision`, about once in 100 million runs). |
| username, login | `user_<16 hex>` | When the login was the email, the fake email. Unique columns stay unique. |
| nicename | the fake login in lower case | At most 50 bytes. |
| first, last, full and display name | from the locale's lists, by hash (`Fergus Hunter`) | `pseudonymise.locale`: `en` (default), `de`, `fr`, `es`, `nl`, `it`. ASCII, so latin1 tables can't break. |
| phone | `+1 555 01x xxxx` | Echo's test range, which Echo never sends to. `phone_style: country` keeps the country's fictional range where one exists (UK `+44 7700 900xxx`) and falls back to the `+1 555 01` range. |
| street, city, company | from the locale's lists (`621 Vale Crescent`, `Woking`) | |
| postcode | the original's shape: digits to digits, letters to letters (`97214` to `99028`) | |
| country, state | kept by default | `pseudonymise.keep` lists the address parts kept. |
| IP address | IPv4 in `192.0.2.0/24`, `198.51.100.0/24`, `203.0.113.0/24`; IPv6 in `2001:db8::/32` | The documentation ranges: never routed. |
| user agent | `Mozilla/5.0 (Roadie staging copy)` | |
| date of birth | shifted by up to 180 days, year kept | Gravity Forms date fields are treated as dates of birth. |
| personal URL (`user_url`) | `https://u<12 hex>.example.invalid/` | Production's own URL becomes the staging URL instead. |
| card number | `XXXX` in free text | Cards don't belong in a database at all; verify reports any it finds. |

A fake never exceeds its column's size: it is shortened deterministically from the schema's limit (`user_login` 60 bytes, `user_email` 100), so MySQL's strict mode never truncates it. Every fake format is one `verify` never flags, which is how a Roadie copy can pass its own verify.

## Consistency and the key

Each fake is computed from a keyed hash: `HMAC-SHA256(key, kind and the normalised value)`. Normalised means emails lower-cased and trimmed, phones as digits with their `+`, names in Unicode NFC and case-folded, IPs in canonical form. So `Jonas.Jablonski45@Gmail.com` and `jonas.jablonski45@gmail.com` get one fake. A person keeps one fake in every table of a run.

`pseudonymise.consistency` decides whether a person keeps that fake across runs:

| Setting | Key | Effect |
|---|---|---|
| `per-run` (default) | 32 random bytes in memory, forgotten when the run ends | Each refresh gives everyone new fakes. Nobody can link two copies or reverse a fake. |
| `stable` (or `--keep-key`) | `ROADIE_KEY` (base64 of 32 bytes), else `.roadie/key` (0600, created on first use, git-excluded) | Each refresh gives each person the same fake, so bookmarks, test accounts and saved searches keep working. |

A stable key is a re-identification key: anyone with the key and a production email can compute that person's fake and find them in the copy. Keep it like a password; in CI keep it in a secret as `ROADIE_KEY`. The key never goes into the copy, the plan, the report or a log; the report holds only its fingerprint (`null` for per-run). Manage it with `config key`:

```text
$ rhythm roadie config key show
No consistency key is kept: each copy uses a new one (consistency: per-run). --keep-key or consistency: stable keeps one in .roadie/key.

$ rhythm roadie config key rotate --force
New consistency key roadie-key:r3omtrinzuea in /home/ann/shop/.roadie/key. Each person gets a new fake on the next copy.

$ rhythm roadie config key show
roadie-key:r3omtrinzuea from /home/ann/shop/.roadie/key

$ ROADIE_KEY=... rhythm roadie config key show
roadie-key:5l4726x3hgkw from ROADIE_KEY (it wins over /home/ann/shop/.roadie/key)

$ rhythm roadie config key forget --force
Removed /home/ann/shop/.roadie/key. Copies use a new key each run until one is kept again.
```

On a terminal `rotate` and `forget` ask first; `--force` is for scripts. A copy with a new key re-maps every person; it never mixes fakes of two keys in one copy.

## Passwords

`pseudonymise.passwords: reset` (the default) gives every user who isn't kept an unusable hash. WordPress gets `$P$roadie` followed by 22 random characters, which phpass never matches; Laravel and generic `password` columns get bcrypt of 32 random bytes. Nobody can log in to the copy with a production password. The hashes are no use to anyone who takes the copy.

To log in, keep your own accounts with `--keep-users` (or `keep_users:`), globs over logins and emails: `--keep-users 'admin,*@agency.example'`. A kept user stays as they are in every table that holds their email or ID, password hash included. The report counts them (`info.kept-users`). `--admin-password-env ROADIE_ADMIN_PASSWORD` sets each kept user's password from that variable instead (only `ROADIE_` names). With no kept user, the copy's last screen names a fake administrator to set a password for:

```text
  No user was kept, so nobody can log in with a real password. Set one: wp
  user update user_ec55e2810c05a5ab --user_pass=<new password>
```

WordPress sessions (`session_tokens`), application passwords and `user_activation_key` are removed; Laravel's `remember_token` and two-factor secrets are set to NULL.

## Free text

Order notes, comments, form messages, mail log bodies and anything else under a `scrub text` rule keep their words. Roadie replaces in them:

- every email, strong-format phone (a `+` and country code, a `tel:` link, a national format with separators), public IP and card number, whoever it belongs to;
- every name, street, address line and postcode it saw in a structured column in pass 1 (so `Ship to 111 Kingsmead Road, Dublin D06 F2X3.` loses the street and postcode; a city on its own stays);
- a street written as a number, capitalised words and a street word (`457 Sabafoben Close`).

What it can't find is a name or an address it never saw in a structured column and that doesn't look like one, written in prose: "my neighbour Marta will collect it". The report's "Not covered" section says so every time. `free_text: blank` empties those columns instead; `keep` leaves them.

`--one-pass` skips pass 1's name index: faster, with names in free text left as they are (the report says so).

## Values no rule names

Plugins keep customer data in places no profile names: their settings, cron arguments, a JSON blob. Two sweeps catch the common cases on top of the rules:

- **known-values.** A whole value anywhere that equals a production email, phone or personal URL seen in pass 1 gets that value's fake. It is counted in the report's changes under `known-values`.
- **uncovered-values.** A leaf of a serialized or JSON value that no rule covers gets a fake by its key (`phone`, `mobile`, `billing_phone`, `first_name`, `last_name`, `full_name`, `display_name`, `street`, `address_1`, `postcode`, `zip` and the like, or `name` under `customer`, `billing`, `shipping`, `user` and similar parents). Any leaf also gets fakes for its personal emails, strong-format phones and public IPs. A street in an options or settings table is the business's own and stays. Counted under `uncovered-values`.

Columns that look personal and have no rule at all stop the copy until you add one ([uncovered columns](Roadie-Copy-And-Prepare#uncovered-columns)). The shop's own name words (`Shop Owner` on `shop.example.com`) and WordPress's system authors (`WooCommerce`, `WordPress`, `Action Scheduler`, `WP-CLI`, `cron`) are not treated as people's names in free text.

## Serialized, JSON and encoded values

Roadie parses PHP serialized values and JSON, rewrites the strings inside them and fixes every length, so `unserialize()` still works (a test runs real PHP over the output). Objects (`O:`, `C:`) are kept as opaque bytes and never instantiated. JSON inside serialized values and serialized values inside JSON are followed. Values a dump writes in hex (`mysqldump --hex-blob`) are decoded, rewritten and written back. Addresses written with an encoded at sign (`%40`, `&#64;`) get the fake their plain form gets, in the same form.

A value that looks serialized but doesn't parse can't be rewritten without breaking it. When it holds personal-looking text, `pseudonymise.on_unparseable: blank` (default) empties it and reports `data.unparseable`; `fail` stops the copy (exit 3). The same goes for a value a dump writes as an SQL expression (`'jane' '@shop.com'`) in a column a rule changes.

Laravel encrypted casts and encrypted job payloads can't be read by anyone without `APP_KEY`; they are listed as `data.encrypted` (info) and become unreadable when the copy gets a new key (`--keep-app-key` keeps it).

## What pseudonymised means here

- A copy that passes `verify` is one where Roadie's checks found no real-looking person, live key or production URL. It is not a statement that the copy holds no personal data.
- Hashes of real values are checked too: exact mode looks for md5 and sha256 of every replaced value. A test makes sure no md5, sha1 or sha256 of an input email appears in an output.
- Production's data is used only to compute fakes in memory; the known-value set holds keyed 8-byte hashes, dropped when the run ends.

---
<sub>← [Profiles](Roadie-Profiles) · [Home](Home) · [Live keys](Roadie-Live-Keys) →</sub>
