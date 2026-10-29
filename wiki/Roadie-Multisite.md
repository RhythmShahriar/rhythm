A WordPress network keeps every site's tables in one database (`wp_options`, `wp_2_options`, `wp_11_options`) plus the network's own (`wp_blogs`, `wp_site`, `wp_sitemeta`, `wp_signups`). Roadie copies the whole network or chosen sites, gives each site's host its staging host and points the network's constants at staging. This page covers subdomain and subdirectory networks, mapped domains and copying some sites only.

## What changes

The `multisite` profile adds the network tables to the WordPress rules: `blogs.domain` and `path` and `site.domain` get their staging hosts, `sitemeta` has its `admin_email` and `siteurl` rewritten and its `site_admins` logins faked, `signups` (email, login, meta), `registration_log` (email, IP) and `blogmeta` are pseudonymised. Every rule written `{prefix}{site}...` applies to each site's tables, so `wp_2_comments` and `wp_11_comments` are covered as `wp_comments` is (and `wp_1_` is never confused with `wp_11_`). Plugins' profiles apply per site the same way: WooCommerce on site 3 only is found on site 3. `rhythm roadie profiles multisite` lists the rules.

On a site target the kill switch also sets `DOMAIN_CURRENT_SITE` and `PATH_CURRENT_SITE` in `wp-config.php` to the staging URL's host (with any port) and path, in the same step that rewrites `WP_HOME` and `WP_SITEURL`. The plan for a copy into a staging network whose `wp-config.php` still names production's host (here without a terminal, so plain):

```text
FILE EDITS  12
- wp-config.php: WP_ENVIRONMENT_TYPE is already 'staging'
- wp-config.php: define( 'DISABLE_WP_CRON', true );
- wp-content/mu-plugins/roadie-staging.php: woocommerce_subscriptions_is_duplicate_site returns true; wc_subscriptions_siteurl left as it is
- wp-config.php: DOMAIN_CURRENT_SITE to staging.shop.test
- wp-config.php: PATH_CURRENT_SITE to /
- wp-config.php: the eight salts (AUTH_KEY to NONCE_SALT) regenerated, 64 random characters each
...
```

## Subdomains and mapped domains

The example network has three sites: `shop.example.com`, `fr.shop.example.com` and a shop on its own domain, `gifts.example.net`:

```text
(1,1,'shop.example.com','/',...),(2,1,'fr.shop.example.com','/',...),(3,1,'gifts.example.net','/',...)
```

A subsite under production's host maps by suffix: `fr.shop.example.com` becomes `fr.staging.shop.test` without being asked. A mapped domain can't be guessed, so the copy stops before a write until you name its staging host:

```sh
rhythm roadie copy --from prod.sql --to staging.sql --url https://staging.shop.test
```

```text
url-map: gifts.example.net has no staging URL; add --url-map gifts.example.net=gifts.staging.shop.test.

rhythm-roadie explain url-map
```

Exit 2. With the map, the plan names every site:

```sh
rhythm roadie plan --from prod.sql --to staging.sql --url https://staging.shop.test --url-map gifts.example.net=gifts.staging.shop.test
```

```text
  ♪ rhythm roadie 1.0.0                                 plan · nothing changes
  ────────────────────────────────────────────────────────────────────────────

  ▸ Source        prod.sql (40.5 KB, 12 tables)
  ▸ Target        staging.sql
  ▸ Stack         WordPress multisite (3 sites) + WooCommerce
  ▸ URLs          shop.example.com (and 2 more) → https://staging.shop.test
                  (serialized and JSON safe)
  ▸ Consistency   per-run key: each real person gets one fake, across every
                  table

  DATA  12 tables ────────────────────────────────────────────────────────────

    wp_2_comments                    25 rows   comment_author,
                                               comment_author_IP,
                                               comment_author_email → fake;
                                               comment_content → URLs
                                               rewritten
    wp_2_options                      5 rows   option_value → URLs rewritten
    wp_3_comments                    25 rows   comment_author,
                                               comment_author_IP,
                                               comment_author_email → fake;
                                               comment_content → URLs
                                               rewritten
    wp_3_options                      5 rows   option_value → URLs rewritten
    wp_3_wc_order_addresses          25 rows   address_1, city, email,
                                               first_name, last_name, phone,
                                               postcode → fake
    wp_3_wc_orders                   25 rows   billing_email, ip_address →
                                               fake
    wp_blogs                          3 rows   domain → URLs rewritten
    wp_comments                      25 rows   comment_author,
                                               comment_author_IP,
                                               comment_author_email → fake;
                                               comment_content → URLs
                                               rewritten
    wp_options                        5 rows   option_value → URLs rewritten
    wp_site                            1 row   domain → URLs rewritten
    · ... 2 more (rhythm-roadie plan -v lists them)

  ALSO ───────────────────────────────────────────────────────────────────────

    · 93 production URLs rewritten in 9 tables
...
 ╭────────────────────────────────────────────────────╮
 │  PLAN   ready: nothing changes until you run copy  │
 ╰────────────────────────────────────────────────────╯
```

The copy passes and the network's hosts are the staging's:

```text
  ✓ Data           728 values pseudonymised in 12 tables, 93 URLs rewritten
  ✓ Keys           no live keys found
  ✓ Written        staging.sql (40.7 KB)
...
 │  PASS   staging.shop.test is a pseudonymised copy  │
```

```text
INSERT INTO `wp_blogs` VALUES (1,1,'staging.shop.test','/',...),(2,1,'fr.staging.shop.test','/',...),(3,1,'gifts.staging.shop.test','/',...)
```

Put the map in the config so every refresh uses it:

```yaml
url: https://staging.shop.test
url_map: {"gifts.example.net": "gifts.staging.shop.test"}
```

A staging network on subdomains needs a wildcard DNS entry and certificate for `*.staging.shop.test` (or one per subsite), as production has. Roadie reads the network's hosts from `wp_blogs`, `wp_site` and `wp_sitemeta` in any order a dump writes them.

## Subdirectories

A subdirectory network (`shop.example.com/fr/`) has one host, so `--url` covers every site: `/fr/` stays `/fr/` under the staging host. A staging under a path (`https://staging.agency.test/shop/`) sets `PATH_CURRENT_SITE` to `/shop/`.

## Some sites only

`--site <id>` (repeatable) copies only those sites. The other sites' tables are dropped from the copy, never copied, so their customers don't reach staging; users are limited to members of the chosen sites plus the super admins. The plan says which tables go:

```sh
rhythm roadie plan --from subdir.sql --to sub-staging.sql --url https://staging.shop.test --site 2
```

```text
DATA  13 tables
wp_11_comments (30 rows): left out of the copy
wp_11_options (5 rows): left out of the copy
wp_2_comments (40 rows): comment_author, comment_author_IP, comment_author_email → fake; comment_content → URLs rewritten
wp_2_options (5 rows): option_value → URLs rewritten
wp_2_wc_order_addresses (40 rows): address_1, city, email, first_name, last_name, phone, postcode → fake
wp_2_wc_orders (40 rows): billing_email, ip_address → fake
wp_blogs (3 rows): domain → URLs rewritten
wp_comments (30 rows): comment_author, comment_author_IP, comment_author_email → fake; comment_content → URLs rewritten
wp_options (5 rows): option_value → URLs rewritten
wp_site (1 row): domain → URLs rewritten
- ... 3 more (rhythm-roadie plan -v lists them)
```

(That plan ran without a terminal, so it is plain text.) The main site's tables stay: the network needs them to boot.

## Verify on a network

`verify` scans every site's tables and reports findings by table, so `wp_3_wc_orders.billing_email` names the site. `url.production` covers every production host of the network, mapped domains included, once they are in `production:` or the source's `wp_blogs`.

---
<sub>← [Generic SQL and Postgres](Roadie-Generic-SQL-And-Postgres) · [Home](Home) · [CI](Roadie-CI) →</sub>
