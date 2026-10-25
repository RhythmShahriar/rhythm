An order email that still arrives but without its PDF invoice is a broken order email. Echo records every file an email attaches (its name, type, size and SHA-256, never its content) and `compare` reports a file that's no longer attached, one that came out empty and one whose type or size changed. This page covers what's recorded, where it shows and how attachments are compared.

## What's recorded

Every captured email lists its files in `attachments`:

```sh
rhythm echo show 68f8c3c0ca4a
```

```json
      "attachments": [
        {
          "name": "invoice-1042.pdf",
          "type": "application/pdf",
          "size": 84043,
          "sha256": "54727fcc3a09a85f0fc979479ce0f498522ed161b84eed240c8cd79746563543"
        }
      ]
```

| Field | Means |
|---|---|
| `name` | The file name, decoded from either encoding mail clients use for names outside ASCII. A part named only in its `Content-Type` counts. |
| `type` | The MIME type the mailer gave, else guessed from the extension. |
| `size` | Bytes, decoded. |
| `sha256` | The hash of the decoded content, so two runs can tell the same file from another. |
| `inline` | `true` for a file shown in the body, such as a logo with a `Content-ID`. |

The content is never stored, in memory, on disk or in a report: a binary part of the raw message is replaced by one line, `[attachment invoice-42.pdf application/pdf 84211 sha256:<64 hex>]`. An email without files has no `attachments` key. Echo keeps up to 100 attachments per email.

Where the facts come from depends on how the email reached Echo:

| Source | Attachments |
|---|---|
| The SMTP sink, `rhythm-echo sendmail`, an API mailer's raw MIME | read from the MIME message, nested parts included |
| WordPress connector | from PHPMailer's attachments (string attachments too) and from `wp_mail()`'s file paths; a file it can't read gets its name and type only |
| Laravel connector | from the message when its raw MIME is too large to send (over 2 MiB) |
| Node connector | from Nodemailer's `attachments` (content, local paths, `data:` URIs); a stream or a remote URL gets its name and type only |

## Where it shows

`ls -v` and `watch -v` add a line under each email with files:

```text
c4fdbeb75977  14:41:08 email to jamie@shop.test: Your invoice for order 1042 [faked]
  attaches invoice-1042.pdf (application/pdf, 0 bytes)
```

Echo Inbox has an **Attachments** tab and a `1 file` badge; its search takes `has:attachment`. The Markdown report adds a table with each email's files; the HTML report lists them under each email's subject. Sizes read `1 byte`, `512 bytes`, `84 KB` (1000-based) and `1.4 MB`.

Mailpit clients see them as Mailpit shows them: `GET /api/v1/message/{ID}` lists `Attachments` and `Inline` with `FileName`, `ContentType`, `Size` and `Checksums.SHA256`. The summary's `Attachments` counts the files offered as downloads. Since Echo keeps no content, there's no `PartID` and `GET /api/v1/message/{ID}/part/{PartID}` answers `404` with `attachment content is not stored: Echo keeps each file's name, type, size and checksum only`.

## How they're compared

Attachments are matched by name, ignoring case and numbers, so `invoice-1042.pdf` today is the same file as `invoice-1043.pdf` tomorrow.

| Check | Class | Title | Before → after |
|---|---|---|---|
| `message/attachment-lost` | BROKE | Email to jamie@shop.test no longer attaches invoice-1042.pdf | `invoice-1042.pdf (application/pdf, 84 KB)` → `not attached` |
| `message/attachment-changed` | BROKE | Email to jamie@shop.test: attachment invoice-1042.pdf is empty | `84 KB` → `0 bytes` |
| `message/attachment-changed` | CHANGED | …: attachment invoice-1042.pdf changed | the sizes and the first 12 characters of the hashes |
| `message/attachment-changed` | CHANGED | …: attachment invoice-1042.pdf changed type | the types |
| `message/attachment-new` | NEW | …now attaches receipt-42.pdf | `not attached` → the file |

The invoice plugin stopped attaching the PDF after an update:

```text
[BROKE] Email to jamie@shop.test no longer attaches invoice-1042.pdf - Invoice
  invoice-1042.pdf (application/pdf, 84 KB) → not attached
  fix: Something stopped attaching the file: check the plugin's attachment setting and that the file is still made.
```

The PDF library failed, so the file is there but empty:

```text
[BROKE] Email to jamie@shop.test: attachment invoice-1042.pdf is empty - Invoice
  84 KB → 0 bytes
  fix: The file is attached but has nothing in it: check how it's made (a PDF library error, a missing template).
```

Generated files differ on every run: a PDF invoice embeds its creation time and an ID, so its hash never matches. A file with the same name whose bytes changed is therefore reported as changed only when its size moved by more than a tenth; an empty file and a new type are always reported. A file renamed only by its number is checked only for being empty, since it's another order's document. When a site's files are made fresh every time and you don't want even the size check, turn it off for that project:

```yaml
# .echo/config.yaml
checks:
  message/attachment-changed: off
```

See [Checks and rules](Echo-Checks-And-Rules#tuning-a-check). Snapshots taken before Echo 1.1 have no `attachments` list; their files are read from the stored MIME body instead, so comparing with them still works.

---
<sub>← [Checks and rules](Echo-Checks-And-Rules) · [Home](Home) · [Actions and compare](Echo-Actions-And-Compare) →</sub>
