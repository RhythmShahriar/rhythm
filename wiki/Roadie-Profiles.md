A profile is the set of rules Roadie applies to one stack or plugin: which tables and columns hold personal data, what each one becomes and which tables are emptied. Roadie 1.0 ships 24 profiles. This page lists every one with its detector, its rules and what it was checked against, exactly as `rhythm roadie profiles <name>` prints them for 1.0.0.

## How profiles apply

With `profiles: [auto]` (the default) Roadie turns a profile on when it finds its tables, options or plugin files: `plan` and the copy's `Stack` line name them (`WordPress + Action Scheduler + WooCommerce (HPOS)`). Name them instead to apply exactly those: `profiles: [wordpress, woocommerce, gravity-forms]`. `stack: sql` applies none: only your `columns:` rules.

`{prefix}` is the table prefix (`$table_prefix` from `wp-config.php`, or detected from the dump) and `{site}` is a multisite blog's `N_` part, so one rule covers `wp_users`, `wp_2_comments` and `wp_11_comments`. A rule with `(meta_key = ...)` applies to the rows where that column matches (globs allowed); `→ serialized:ip` and `→ json:...` reach inside a serialized or JSON value.

| Rule | What happens |
|---|---|
| `fake <kind>` | A consistent fake of that kind: email, username, nicename, first_name, last_name, full_name, phone, street, city, postcode, company, ip, user_agent, url, dob. See [Pseudonymisation](Roadie-Pseudonymisation). |
| `scrub text` | The text stays; emails, phones, cards, IPs, streets, postcodes and known names in it are replaced by their fakes. |
| `blank`, `null` | Emptied, or set to NULL. |
| `reset-password` | An unusable hash, unless the user is kept. |
| `delete-row` | The row is left out (session tokens, application passwords, transients). |
| `rewrite-url` | Production's URLs to the staging URL. |
| `fixed` | A set value, such as a webhook's `status` to `paused`. |
| `keep`, `kept (keep: country)` | Left as it is: by the rule, or because `pseudonymise.keep` keeps that address part. |
| `truncated:` | The table's structure is copied, its rows aren't. |

Personal values outside every rule are not left to chance: a production email, phone or URL found whole in an uncovered value gets its fake (`known-values`). Plugin settings and cron arguments get fakes for their phone, name, street and email keys (`uncovered-values`). Any column that still looks personal stops the copy until a rule covers it. See [Pseudonymisation](Roadie-Pseudonymisation#values-no-rule-names).

To change a profile's rule for one column, list the column under `columns:` in `.roadie/config.yaml` with the rule you want: your rule wins. `drop:` and `truncate:` leave tables out. `rhythm roadie config show` prints every rule that may apply to your project. `explain <profile>` says what a profile is for; `profiles <name> --json` gives the rules as data.

## The list

```sh
rhythm roadie profiles
```

```text
Profiles (pack 1.0.0):

  action-scheduler           Action Scheduler              docs: https://raw.githubusercontent.com/woocommerce/action-scheduler/3.9.2/classes/schema/ActionScheduler_StoreSchema.php
  automatewoo                AutomateWoo                   docs: https://woocommerce.com/document/automatewoo/regenerate-automatewoo-database-tables/
  contact-form-7-flamingo    Contact Form 7 with Flamingo  docs: https://plugins.svn.wordpress.org/flamingo/trunk/includes/class-inbound-message.php
  edd                        Easy Digital Downloads        docs: https://github.com/awesomemotive/easy-digital-downloads/tree/main/includes/database/schemas
  email-log                  Email Log                     docs: https://plugins.svn.wordpress.org/email-log/trunk/include/Core/DB/TableManager.php
  fluent-forms               Fluent Forms                  docs: https://plugins.svn.wordpress.org/fluentform/trunk/database/Migrations/Submissions.php
  fluent-smtp                FluentSMTP                    docs: https://plugins.svn.wordpress.org/fluent-smtp/trunk/database/migrations/EmailLogs.php
  gravity-forms              Gravity Forms                 docs: https://docs.gravityforms.com/database-storage-structure-reference/
  laravel                    Laravel                       docs: https://github.com/laravel/laravel/blob/12.x/database/migrations/0001_01_01_000000_create_users_table.php
  laravel-cashier            Laravel Cashier               docs: https://github.com/laravel/cashier-stripe/tree/15.x/database/migrations
  learndash                  LearnDash                     docs: https://plugins.svn.wordpress.org/wp-pro-quiz/trunk/lib/helper/WpProQuiz_Helper_DbUpgrade.php
  mailpoet                   MailPoet                      docs: https://github.com/mailpoet/mailpoet/blob/trunk/mailpoet/lib/Entities/SubscriberEntity.php
  memberpress                MemberPress                   docs: https://memberpress.com/docs/where-is-data-stored/
  multisite                  WordPress multisite           docs: https://raw.githubusercontent.com/WordPress/WordPress/6.8-branch/wp-admin/includes/schema.php
  ninja-forms                Ninja Forms                   docs: https://plugins.svn.wordpress.org/ninja-forms/trunk/includes/Database/Migrations/Fields.php
  post-smtp                  Post SMTP                     docs: https://plugins.svn.wordpress.org/post-smtp/trunk/Postman/Postman-Email-Log/PostmanEmailLogService.php
  sql                        Generic SQL                   config: the tables columns: lists
  woocommerce                WooCommerce                   docs: https://raw.githubusercontent.com/woocommerce/woocommerce/9.8.0/plugins/woocommerce/includes/class-wc-install.php
  woocommerce-hpos           WooCommerce HPOS orders       docs: https://raw.githubusercontent.com/woocommerce/woocommerce/9.8.0/plugins/woocommerce/src/Internal/DataStores/Orders/OrdersTableDataStore.php
  woocommerce-subscriptions  WooCommerce Subscriptions     docs: https://woocommerce.com/document/subscriptions-handles-staging-sites/
  wordpress                  WordPress core                docs: https://raw.githubusercontent.com/WordPress/WordPress/6.8-branch/wp-admin/includes/schema.php
  wp-mail-logging            WP Mail Logging               docs: https://plugins.svn.wordpress.org/wp-mail-logging/trunk/src/Model/WPML_Mail.php
  wp-mail-smtp               WP Mail SMTP                  docs: https://wpmailsmtp.com/docs/how-to-set-up-email-logging/
  wpforms                    WPForms                       docs: https://plugins.svn.wordpress.org/wpforms-lite/trunk/src/Db/Payments/Payment.php

rhythm-roadie profiles <name> lists one profile's rules.
```

"Checked against" is the source each profile's tables and columns were taken from: the plugin's own schema file at that version, or its documentation. Every rule has a positive and a negative test fixture. `rhythm roadie update` downloads a newer signed profile pack when one is released (see [CLI reference](Roadie-CLI-Reference#update)); reports record the pack version they ran with.

## WordPress

### wordpress

WordPress core: users, usermeta, comments and order notes, commentmeta, options (admin email, siteurl, home, mailer settings) and transients. user_pass gets an unusable hash unless the user is kept.

```text
WordPress core (wordpress)
checked against: docs: https://raw.githubusercontent.com/WordPress/WordPress/6.8-branch/wp-admin/includes/schema.php
detected by: a table of {prefix}options

  TABLE                      COLUMN                                                             RULE
  {prefix}users              user_email                                                         fake email
  {prefix}users              user_login                                                         fake username
  {prefix}users              user_nicename                                                      fake nicename
  {prefix}users              display_name                                                       fake full_name
  {prefix}users              user_url                                                           fake url
  {prefix}users              user_pass                                                          reset-password
  {prefix}users              user_activation_key                                                blank
  {prefix}usermeta           meta_value (meta_key = first_name)                                 fake first_name
  {prefix}usermeta           meta_value (meta_key = last_name)                                  fake last_name
  {prefix}usermeta           meta_value (meta_key = nickname)                                   fake username
  {prefix}usermeta           meta_value (meta_key = description)                                scrub text
  {prefix}usermeta           meta_value (meta_key = session_tokens)                             delete-row
  {prefix}usermeta           meta_value (meta_key = _application_passwords)                     delete-row
  {prefix}usermeta           meta_value (meta_key = wc_last_active)                             keep
  {prefix}usermeta           meta_value → serialized:ip (meta_key = community-events-location)  fake ip
  {prefix}usermeta           meta_value (meta_key = billing_email)                              fake email
  {prefix}usermeta           meta_value (meta_key = billing_phone)                              fake phone
  {prefix}usermeta           meta_value (meta_key = billing_first_name)                         fake first_name
  {prefix}usermeta           meta_value (meta_key = billing_last_name)                          fake last_name
  {prefix}usermeta           meta_value (meta_key = billing_company)                            fake company
  {prefix}usermeta           meta_value (meta_key = billing_address_1)                          fake street
  {prefix}usermeta           meta_value (meta_key = billing_address_2)                          fake street
  {prefix}usermeta           meta_value (meta_key = billing_city)                               fake city
  {prefix}usermeta           meta_value (meta_key = billing_postcode)                           fake postcode
  {prefix}usermeta           meta_value (meta_key = billing_country)                            kept (keep: country)
  {prefix}usermeta           meta_value (meta_key = billing_state)                              kept (keep: state)
  {prefix}usermeta           meta_value (meta_key = shipping_phone)                             fake phone
  {prefix}usermeta           meta_value (meta_key = shipping_first_name)                        fake first_name
  {prefix}usermeta           meta_value (meta_key = shipping_last_name)                         fake last_name
  {prefix}usermeta           meta_value (meta_key = shipping_company)                           fake company
  {prefix}usermeta           meta_value (meta_key = shipping_address_1)                         fake street
  {prefix}usermeta           meta_value (meta_key = shipping_address_2)                         fake street
  {prefix}usermeta           meta_value (meta_key = shipping_city)                              fake city
  {prefix}usermeta           meta_value (meta_key = shipping_postcode)                          fake postcode
  {prefix}usermeta           meta_value (meta_key = shipping_country)                           kept (keep: country)
  {prefix}usermeta           meta_value (meta_key = shipping_state)                             kept (keep: state)
  {prefix}{site}comments     comment_author                                                     fake full_name
  {prefix}{site}comments     comment_author_email                                               fake email
  {prefix}{site}comments     comment_author_url                                                 fake url
  {prefix}{site}comments     comment_author_IP                                                  fake ip
  {prefix}{site}comments     comment_agent                                                      fake user_agent
  {prefix}{site}comments     comment_content                                                    scrub text
  {prefix}{site}commentmeta  meta_value                                                         scrub text
  {prefix}{site}posts        post_title (post_type = user_request)                              fake email
  {prefix}{site}posts        post_content (post_type = user_request)                            scrub text
  {prefix}{site}options      option_value (option_name = admin_email)                           fake email
  {prefix}{site}options      option_value (option_name = new_admin_email)                       fake email
  {prefix}{site}options      option_value (option_name = siteurl)                               rewrite-url
  {prefix}{site}options      option_value (option_name = home)                                  rewrite-url
  {prefix}{site}options      option_value (option_name = wc_subscriptions_siteurl)              keep
  {prefix}{site}options      option_value (option_name = _transient_*)                          delete-row
  {prefix}{site}options      option_value (option_name = _site_transient_*)                     delete-row

rhythm-roadie explain wordpress says why.
```

### multisite

WordPress multisite: blogs and site hosts rewritten, sitemeta (admin email, site admins, siteurl), signups, registration_log and blogmeta, for every wp_N_ site table.

```text
WordPress multisite (multisite)
checked against: docs: https://raw.githubusercontent.com/WordPress/WordPress/6.8-branch/wp-admin/includes/schema.php
detected by: a table of {prefix}blogs, {prefix}sitemeta

  TABLE                     COLUMN                                              RULE
  {prefix}blogs             domain                                              rewrite-url
  {prefix}blogs             path                                                rewrite-url
  {prefix}site              domain                                              rewrite-url
  {prefix}site              path                                                rewrite-url
  {prefix}sitemeta          meta_value (meta_key = admin_email)                 fake email
  {prefix}sitemeta          meta_value → serialized:* (meta_key = site_admins)  fake username
  {prefix}sitemeta          meta_value (meta_key = siteurl)                     rewrite-url
  {prefix}sitemeta          meta_value (meta_key = *domain*)                    rewrite-url
  {prefix}signups           user_email                                          fake email
  {prefix}signups           user_login                                          fake username
  {prefix}signups           meta                                                scrub text
  {prefix}signups           activation_key                                      blank
  {prefix}registration_log  email                                               fake email
  {prefix}registration_log  IP                                                  fake ip
  {prefix}blogmeta          meta_value                                          scrub text

rhythm-roadie explain multisite says why.
```

## WooCommerce

### woocommerce

WooCommerce legacy orders in posts and postmeta (_billing_* and _shipping_*), customer lookup, sessions and payment tokens truncated, download logs, webhooks paused and API keys truncated.

```text
WooCommerce (woocommerce)
checked against: docs: https://raw.githubusercontent.com/woocommerce/woocommerce/9.8.0/plugins/woocommerce/includes/class-wc-install.php
detected by: a table of {prefix}{site}woocommerce_order_items, {prefix}{site}wc_customer_lookup, {prefix}{site}woocommerce_sessions, {prefix}{site}wc_orders, {prefix}{site}wc_order_stats, {prefix}{site}woocommerce_api_keys; or an option of woocommerce_version, woocommerce_db_version, woocommerce_custom_orders_table_enabled; or a file of wp-content/plugins/woocommerce/woocommerce.php

  TABLE                                                       COLUMN                                           RULE
  {prefix}{site}postmeta                                      meta_value (meta_key = _billing_email)           fake email
  {prefix}{site}postmeta                                      meta_value (meta_key = _billing_phone)           fake phone
  {prefix}{site}postmeta                                      meta_value (meta_key = _billing_first_name)      fake first_name
  {prefix}{site}postmeta                                      meta_value (meta_key = _billing_last_name)       fake last_name
  {prefix}{site}postmeta                                      meta_value (meta_key = _billing_company)         fake company
  {prefix}{site}postmeta                                      meta_value (meta_key = _billing_address_1)       fake street
  {prefix}{site}postmeta                                      meta_value (meta_key = _billing_address_2)       fake street
  {prefix}{site}postmeta                                      meta_value (meta_key = _billing_city)            fake city
  {prefix}{site}postmeta                                      meta_value (meta_key = _billing_postcode)        fake postcode
  {prefix}{site}postmeta                                      meta_value (meta_key = _billing_country)         kept (keep: country)
  {prefix}{site}postmeta                                      meta_value (meta_key = _billing_state)           kept (keep: state)
  {prefix}{site}postmeta                                      meta_value (meta_key = _billing_address_index)   scrub text
  {prefix}{site}postmeta                                      meta_value (meta_key = _shipping_phone)          fake phone
  {prefix}{site}postmeta                                      meta_value (meta_key = _shipping_first_name)     fake first_name
  {prefix}{site}postmeta                                      meta_value (meta_key = _shipping_last_name)      fake last_name
  {prefix}{site}postmeta                                      meta_value (meta_key = _shipping_company)        fake company
  {prefix}{site}postmeta                                      meta_value (meta_key = _shipping_address_1)      fake street
  {prefix}{site}postmeta                                      meta_value (meta_key = _shipping_address_2)      fake street
  {prefix}{site}postmeta                                      meta_value (meta_key = _shipping_city)           fake city
  {prefix}{site}postmeta                                      meta_value (meta_key = _shipping_postcode)       fake postcode
  {prefix}{site}postmeta                                      meta_value (meta_key = _shipping_country)        kept (keep: country)
  {prefix}{site}postmeta                                      meta_value (meta_key = _shipping_state)          kept (keep: state)
  {prefix}{site}postmeta                                      meta_value (meta_key = _shipping_address_index)  scrub text
  {prefix}{site}postmeta                                      meta_value (meta_key = _customer_ip_address)     fake ip
  {prefix}{site}postmeta                                      meta_value (meta_key = _customer_user_agent)     fake user_agent
  {prefix}{site}postmeta                                      meta_value (meta_key = _stripe_customer_id)      blank
  {prefix}{site}postmeta                                      meta_value (meta_key = _stripe_source_id)        blank
  {prefix}{site}postmeta                                      meta_value (meta_key = _stripe_intent_id)        blank
  {prefix}{site}postmeta                                      meta_value (meta_key = _payment_tokens)          blank
  {prefix}{site}postmeta                                      meta_value (meta_key = _billing_vat_number)      blank
  {prefix}{site}postmeta                                      meta_value (meta_key = _wcpay_*customer*)        blank
  {prefix}usermeta                                            meta_value (meta_key = *_stripe_customer_id*)    blank
  {prefix}usermeta                                            meta_value (meta_key = *_wcpay_customer_id*)     blank
  {prefix}{site}posts                                         post_excerpt (post_type = shop_order*)           scrub text
  {prefix}{site}wc_customer_lookup                            first_name                                       fake first_name
  {prefix}{site}wc_customer_lookup                            last_name                                        fake last_name
  {prefix}{site}wc_customer_lookup                            email                                            fake email
  {prefix}{site}wc_customer_lookup                            username                                         fake username
  {prefix}{site}wc_customer_lookup                            city                                             fake city
  {prefix}{site}wc_customer_lookup                            postcode                                         fake postcode
  {prefix}{site}wc_customer_lookup                            country                                          kept (keep: country)
  {prefix}{site}wc_customer_lookup                            state                                            kept (keep: state)
  {prefix}{site}woocommerce_downloadable_product_permissions  user_email                                       fake email
  {prefix}{site}wc_download_log                               user_ip_address                                  fake ip
  {prefix}{site}woocommerce_log                               message                                          scrub text
  {prefix}{site}woocommerce_log                               context                                          scrub text
  {prefix}{site}wc_webhooks                                   status                                           fixed
  {prefix}{site}wc_webhooks                                   secret                                           blank
  {prefix}{site}wc_webhooks                                   delivery_url                                     keep

truncated: {prefix}{site}woocommerce_sessions, {prefix}{site}woocommerce_payment_tokens, {prefix}{site}woocommerce_payment_tokenmeta, {prefix}{site}woocommerce_api_keys, {prefix}{site}wc_admin_note_actions

rhythm-roadie explain woocommerce says why.
```

### woocommerce-hpos

WooCommerce HPOS tables: wc_orders, wc_order_addresses, wc_orders_meta and wc_order_operational_data, with the address indexes recomputed from the fake parts.

```text
WooCommerce HPOS orders (woocommerce-hpos)
checked against: docs: https://raw.githubusercontent.com/woocommerce/woocommerce/9.8.0/plugins/woocommerce/src/Internal/DataStores/Orders/OrdersTableDataStore.php
detected by: a table of {prefix}{site}wc_orders

  TABLE                                    COLUMN                                           RULE
  {prefix}{site}wc_orders                  billing_email                                    fake email
  {prefix}{site}wc_orders                  ip_address                                       fake ip
  {prefix}{site}wc_orders                  user_agent                                       fake user_agent
  {prefix}{site}wc_orders                  customer_note                                    scrub text
  {prefix}{site}wc_orders                  transaction_id                                   blank
  {prefix}{site}wc_order_addresses         first_name                                       fake first_name
  {prefix}{site}wc_order_addresses         last_name                                        fake last_name
  {prefix}{site}wc_order_addresses         company                                          fake company
  {prefix}{site}wc_order_addresses         address_1                                        fake street
  {prefix}{site}wc_order_addresses         address_2                                        fake street
  {prefix}{site}wc_order_addresses         city                                             fake city
  {prefix}{site}wc_order_addresses         postcode                                         fake postcode
  {prefix}{site}wc_order_addresses         country                                          kept (keep: country)
  {prefix}{site}wc_order_addresses         state                                            kept (keep: state)
  {prefix}{site}wc_order_addresses         email                                            fake email
  {prefix}{site}wc_order_addresses         phone                                            fake phone
  {prefix}{site}wc_order_operational_data  order_key                                        keep
  {prefix}{site}wc_orders_meta             meta_value (meta_key = _billing_email)           fake email
  {prefix}{site}wc_orders_meta             meta_value (meta_key = _billing_phone)           fake phone
  {prefix}{site}wc_orders_meta             meta_value (meta_key = _billing_first_name)      fake first_name
  {prefix}{site}wc_orders_meta             meta_value (meta_key = _billing_last_name)       fake last_name
  {prefix}{site}wc_orders_meta             meta_value (meta_key = _billing_company)         fake company
  {prefix}{site}wc_orders_meta             meta_value (meta_key = _billing_address_1)       fake street
  {prefix}{site}wc_orders_meta             meta_value (meta_key = _billing_address_2)       fake street
  {prefix}{site}wc_orders_meta             meta_value (meta_key = _billing_city)            fake city
  {prefix}{site}wc_orders_meta             meta_value (meta_key = _billing_postcode)        fake postcode
  {prefix}{site}wc_orders_meta             meta_value (meta_key = _billing_country)         kept (keep: country)
  {prefix}{site}wc_orders_meta             meta_value (meta_key = _billing_state)           kept (keep: state)
  {prefix}{site}wc_orders_meta             meta_value (meta_key = _billing_address_index)   scrub text
  {prefix}{site}wc_orders_meta             meta_value (meta_key = _shipping_phone)          fake phone
  {prefix}{site}wc_orders_meta             meta_value (meta_key = _shipping_first_name)     fake first_name
  {prefix}{site}wc_orders_meta             meta_value (meta_key = _shipping_last_name)      fake last_name
  {prefix}{site}wc_orders_meta             meta_value (meta_key = _shipping_company)        fake company
  {prefix}{site}wc_orders_meta             meta_value (meta_key = _shipping_address_1)      fake street
  {prefix}{site}wc_orders_meta             meta_value (meta_key = _shipping_address_2)      fake street
  {prefix}{site}wc_orders_meta             meta_value (meta_key = _shipping_city)           fake city
  {prefix}{site}wc_orders_meta             meta_value (meta_key = _shipping_postcode)       fake postcode
  {prefix}{site}wc_orders_meta             meta_value (meta_key = _shipping_country)        kept (keep: country)
  {prefix}{site}wc_orders_meta             meta_value (meta_key = _shipping_state)          kept (keep: state)
  {prefix}{site}wc_orders_meta             meta_value (meta_key = _shipping_address_index)  scrub text
  {prefix}{site}wc_orders_meta             meta_value (meta_key = _customer_ip_address)     fake ip
  {prefix}{site}wc_orders_meta             meta_value (meta_key = _customer_user_agent)     fake user_agent
  {prefix}{site}wc_orders_meta             meta_value (meta_key = _stripe_customer_id)      blank
  {prefix}{site}wc_orders_meta             meta_value (meta_key = _stripe_source_id)        blank
  {prefix}{site}wc_orders_meta             meta_value (meta_key = _stripe_intent_id)        blank
  {prefix}{site}wc_orders_meta             meta_value (meta_key = _payment_tokens)          blank
  {prefix}{site}wc_orders_meta             meta_value (meta_key = _billing_vat_number)      blank
  {prefix}{site}wc_orders_meta             meta_value (meta_key = _wcpay_*customer*)        blank

rhythm-roadie explain woocommerce-hpos says why.
```

### action-scheduler

Action Scheduler: args and extended_args walked as JSON and normalised again as Action Scheduler stores them; log messages scrubbed.

```text
Action Scheduler (action-scheduler)
checked against: docs: https://raw.githubusercontent.com/woocommerce/action-scheduler/3.9.2/classes/schema/ActionScheduler_StoreSchema.php
detected by: a table of {prefix}{site}actionscheduler_actions

  TABLE                                  COLUMN         RULE
  {prefix}{site}actionscheduler_actions  args           scrub text
  {prefix}{site}actionscheduler_actions  extended_args  scrub text
  {prefix}{site}actionscheduler_logs     message        scrub text

rhythm-roadie explain action-scheduler says why.
```

### woocommerce-subscriptions

WooCommerce Subscriptions: subscription orders by the order rules, gateway customer and token IDs blanked, wc_subscriptions_siteurl never rewritten, staging mode forced.

```text
WooCommerce Subscriptions (woocommerce-subscriptions)
checked against: docs: https://woocommerce.com/document/subscriptions-handles-staging-sites/
detected by: an option of wc_subscriptions_siteurl, woocommerce_subscriptions_is_active; or a file of wp-content/plugins/woocommerce-subscriptions/woocommerce-subscriptions.php, wp-content/plugins/woocommerce-subscriptions-core/woocommerce-subscriptions-core.php

  TABLE                         COLUMN                                                 RULE
  {prefix}{site}postmeta        meta_value (meta_key = _payment_method_token)          blank
  {prefix}{site}wc_orders_meta  meta_value (meta_key = _payment_method_token)          blank
  {prefix}{site}posts           post_excerpt (post_type = shop_subscription)           scrub text
  {prefix}{site}options         option_value (option_name = wc_subscriptions_siteurl)  keep

rhythm-roadie explain woocommerce-subscriptions says why.
```

### automatewoo

AutomateWoo: queue and abandoned carts truncated, guests and customers pseudonymised, log meta scrubbed.

```text
AutomateWoo (automatewoo)
checked against: docs: https://woocommerce.com/document/automatewoo/regenerate-automatewoo-database-tables/
detected by: a table of {prefix}{site}automatewoo_queue, {prefix}{site}automatewoo_guests; or a file of wp-content/plugins/automatewoo/automatewoo.php

  TABLE                                 COLUMN      RULE
  {prefix}{site}automatewoo_guests      email       fake email
  {prefix}{site}automatewoo_guest_meta  meta_value  scrub text
  {prefix}{site}automatewoo_log_meta    meta_value  scrub text

truncated: {prefix}{site}automatewoo_queue, {prefix}{site}automatewoo_queue_meta, {prefix}{site}automatewoo_abandoned_carts, {prefix}{site}automatewoo_customers

rhythm-roadie explain automatewoo says why.
```

## Forms

### gravity-forms

Gravity Forms: entries (IP, user agent, source URL), entry meta by the field types in the form JSON, notes scrubbed, form views truncated.

```text
Gravity Forms (gravity-forms)
checked against: docs: https://docs.gravityforms.com/database-storage-structure-reference/
detected by: a table of {prefix}{site}gf_form, {prefix}{site}gf_entry; or a file of wp-content/plugins/gravityforms/gravityforms.php

  TABLE                         COLUMN          RULE
  {prefix}{site}gf_entry        ip              fake ip
  {prefix}{site}gf_entry        user_agent      fake user_agent
  {prefix}{site}gf_entry        source_url      rewrite-url
  {prefix}{site}gf_entry        transaction_id  blank
  {prefix}{site}gf_entry_meta   meta_value      scrub text
  {prefix}{site}gf_entry_notes  value           scrub text
  {prefix}{site}gf_entry_notes  user_name       fake full_name
  {prefix}{site}gf_form_meta    notifications   scrub text

truncated: {prefix}{site}gf_draft_submissions, {prefix}{site}gf_form_view

rhythm-roadie explain gravity-forms says why.
```

### wpforms

WPForms: entry fields by field type, IP address and user agent.

```text
WPForms (wpforms)
checked against: docs: https://plugins.svn.wordpress.org/wpforms-lite/trunk/src/Db/Payments/Payment.php
detected by: a table of {prefix}{site}wpforms_entries, {prefix}{site}wpforms_payments, {prefix}{site}wpforms_tasks_meta; or a file of wp-content/plugins/wpforms-lite/wpforms.php, wp-content/plugins/wpforms/wpforms.php

  TABLE                               COLUMN           RULE
  {prefix}{site}wpforms_entries       fields           scrub text
  {prefix}{site}wpforms_entries       meta             scrub text
  {prefix}{site}wpforms_entries       ip_address       fake ip
  {prefix}{site}wpforms_entries       user_agent       fake user_agent
  {prefix}{site}wpforms_entries       user_uuid        blank
  {prefix}{site}wpforms_entry_fields  value            scrub text
  {prefix}{site}wpforms_entry_meta    data             scrub text
  {prefix}{site}wpforms_payments      title            fake full_name
  {prefix}{site}wpforms_payments      customer_id      blank
  {prefix}{site}wpforms_payments      transaction_id   blank
  {prefix}{site}wpforms_payments      subscription_id  blank
  {prefix}{site}wpforms_payment_meta  meta_value       scrub text

truncated: {prefix}{site}wpforms_logs

rhythm-roadie explain wpforms says why.
```

### contact-form-7-flamingo

Contact Form 7 with Flamingo: inbound messages and contacts (from email, from name, fields scrubbed, IPs in _meta).

```text
Contact Form 7 with Flamingo (contact-form-7-flamingo)
checked against: docs: https://plugins.svn.wordpress.org/flamingo/trunk/includes/class-inbound-message.php
detected by: an option of wpcf7; or a file of wp-content/plugins/flamingo/flamingo.php

  TABLE                   COLUMN                                                 RULE
  {prefix}{site}posts     post_content (post_type = flamingo_inbound)            scrub text
  {prefix}{site}posts     post_title (post_type = flamingo_inbound)              scrub text
  {prefix}{site}posts     post_title (post_type = flamingo_contact)              fake email
  {prefix}{site}posts     post_name (post_type = flamingo_contact)               fake nicename
  {prefix}{site}postmeta  meta_value (meta_key = _from)                          scrub text
  {prefix}{site}postmeta  meta_value (meta_key = _from_email)                    fake email
  {prefix}{site}postmeta  meta_value (meta_key = _from_name)                     fake full_name
  {prefix}{site}postmeta  meta_value (meta_key = _field_your-email)              fake email
  {prefix}{site}postmeta  meta_value (meta_key = _field_your-name)               fake full_name
  {prefix}{site}postmeta  meta_value (meta_key = _field_*)                       scrub text
  {prefix}{site}postmeta  meta_value → serialized:remote_ip (meta_key = _meta)   fake ip
  {prefix}{site}postmeta  meta_value → serialized:user_agent (meta_key = _meta)  fake user_agent
  {prefix}{site}postmeta  meta_value (meta_key = _email)                         fake email
  {prefix}{site}postmeta  meta_value (meta_key = _name)                          fake full_name
  {prefix}{site}postmeta  meta_value (meta_key = _props)                         scrub text

rhythm-roadie explain contact-form-7-flamingo says why.
```

### ninja-forms

Ninja Forms: submissions in nf3_objects and nf3_object_meta by the field types of the form.

```text
Ninja Forms (ninja-forms)
checked against: docs: https://plugins.svn.wordpress.org/ninja-forms/trunk/includes/Database/Migrations/Fields.php
detected by: a table of {prefix}{site}nf3_forms, {prefix}{site}nf3_fields; or a file of wp-content/plugins/ninja-forms/ninja-forms.php

  TABLE                          COLUMN                            RULE
  {prefix}{site}postmeta         meta_value (meta_key = _field_*)  scrub text
  {prefix}{site}nf3_object_meta  value                             scrub text
  {prefix}{site}nf3_object_meta  meta_value                        scrub text

truncated: {prefix}{site}nf3_chunks

rhythm-roadie explain ninja-forms says why.
```

### fluent-forms

Fluent Forms: submission responses walked as JSON, entry details and IPs.

```text
Fluent Forms (fluent-forms)
checked against: docs: https://plugins.svn.wordpress.org/fluentform/trunk/database/Migrations/Submissions.php
detected by: a table of {prefix}{site}fluentform_forms, {prefix}{site}fluentform_submissions; or a file of wp-content/plugins/fluentform/fluentform.php

  TABLE                                     COLUMN                                                                  RULE
  {prefix}{site}fluentform_submissions      response                                                                scrub text
  {prefix}{site}fluentform_submissions      response → json:email                                                   fake email
  {prefix}{site}fluentform_submissions      response → json:names.first_name                                        fake first_name
  {prefix}{site}fluentform_submissions      response → json:names.middle_name                                       fake first_name
  {prefix}{site}fluentform_submissions      response → json:names.last_name                                         fake last_name
  {prefix}{site}fluentform_submissions      response → json:phone                                                   fake phone
  {prefix}{site}fluentform_submissions      response → json:address_1.address_line_1                                fake street
  {prefix}{site}fluentform_submissions      response → json:address_1.address_line_2                                fake street
  {prefix}{site}fluentform_submissions      response → json:address_1.city                                          fake city
  {prefix}{site}fluentform_submissions      response → json:address_1.zip                                           fake postcode
  {prefix}{site}fluentform_submissions      response → json:address_1.state                                         kept (keep: state)
  {prefix}{site}fluentform_submissions      response → json:address_1.country                                       kept (keep: country)
  {prefix}{site}fluentform_submissions      source_url                                                              rewrite-url
  {prefix}{site}fluentform_submissions      ip                                                                      fake ip
  {prefix}{site}fluentform_submissions      city                                                                    fake city
  {prefix}{site}fluentform_entry_details    field_value (field_name = email)                                        fake email
  {prefix}{site}fluentform_entry_details    field_value (field_name = phone)                                        fake phone
  {prefix}{site}fluentform_entry_details    field_value (field_name = names) (sub_field_name = first_name)          fake first_name
  {prefix}{site}fluentform_entry_details    field_value (field_name = names) (sub_field_name = middle_name)         fake first_name
  {prefix}{site}fluentform_entry_details    field_value (field_name = names) (sub_field_name = last_name)           fake last_name
  {prefix}{site}fluentform_entry_details    field_value (field_name = address_1) (sub_field_name = address_line_1)  fake street
  {prefix}{site}fluentform_entry_details    field_value (field_name = address_1) (sub_field_name = address_line_2)  fake street
  {prefix}{site}fluentform_entry_details    field_value (field_name = address_1) (sub_field_name = city)            fake city
  {prefix}{site}fluentform_entry_details    field_value (field_name = address_1) (sub_field_name = zip)             fake postcode
  {prefix}{site}fluentform_entry_details    field_value (field_name = address_1) (sub_field_name = state)           kept (keep: state)
  {prefix}{site}fluentform_entry_details    field_value (field_name = address_1) (sub_field_name = country)         kept (keep: country)
  {prefix}{site}fluentform_entry_details    field_value                                                             scrub text
  {prefix}{site}fluentform_submission_meta  value                                                                   scrub text
  {prefix}{site}fluentform_logs             description                                                             scrub text

truncated: {prefix}{site}fluentform_form_analytics, {prefix}{site}fluentform_draft_submissions

rhythm-roadie explain fluent-forms says why.
```

## Mail logs

### wp-mail-smtp

WP Mail SMTP: the email log and debug events truncated; the mailer switched to the PHP mailer with its credentials blanked so Echo sees every mail.

```text
WP Mail SMTP (wp-mail-smtp)
checked against: docs: https://wpmailsmtp.com/docs/how-to-set-up-email-logging/
detected by: a table of {prefix}{site}wpmailsmtp_emails_log, {prefix}{site}wpmailsmtp_debug_events; or a file of wp-content/plugins/wp-mail-smtp/wp_mail_smtp.php, wp-content/plugins/wp-mail-smtp-pro/wp_mail_smtp.php

truncated: {prefix}{site}wpmailsmtp_emails_log, {prefix}{site}wpmailsmtp_debug_events

rhythm-roadie explain wp-mail-smtp says why.
```

### post-smtp

Post SMTP: logs and log meta truncated; connections switched off with credentials blanked.

```text
Post SMTP (post-smtp)
checked against: docs: https://plugins.svn.wordpress.org/post-smtp/trunk/Postman/Postman-Email-Log/PostmanEmailLogService.php
detected by: a table of {prefix}{site}post_smtp_logs, {prefix}{site}post_smtp_logmeta; or a file of wp-content/plugins/post-smtp/postman-smtp.php

  TABLE                COLUMN                                        RULE
  {prefix}{site}posts  post_content (post_type = postman_sent_mail)  delete-row

truncated: {prefix}{site}post_smtp_logs, {prefix}{site}post_smtp_logmeta

rhythm-roadie explain post-smtp says why.
```

### fluent-smtp

FluentSMTP: email logs truncated; connections switched off with credentials blanked.

```text
FluentSMTP (fluent-smtp)
checked against: docs: https://plugins.svn.wordpress.org/fluent-smtp/trunk/database/migrations/EmailLogs.php
detected by: a table of {prefix}{site}fsmpt_email_logs; or a file of wp-content/plugins/fluent-smtp/fluent-smtp.php

truncated: {prefix}{site}fsmpt_email_logs

rhythm-roadie explain fluent-smtp says why.
```

### wp-mail-logging

WP Mail Logging: the mail log truncated.

```text
WP Mail Logging (wp-mail-logging)
checked against: docs: https://plugins.svn.wordpress.org/wp-mail-logging/trunk/src/Model/WPML_Mail.php
detected by: a table of {prefix}{site}wpml_mails; or a file of wp-content/plugins/wp-mail-logging/wp-mail-logging.php

truncated: {prefix}{site}wpml_mails

rhythm-roadie explain wp-mail-logging says why.
```

### email-log

Email Log: the log truncated.

```text
Email Log (email-log)
checked against: docs: https://plugins.svn.wordpress.org/email-log/trunk/include/Core/DB/TableManager.php
detected by: a table of {prefix}{site}email_log; or a file of wp-content/plugins/email-log/email-log.php

truncated: {prefix}{site}email_log

rhythm-roadie explain email-log says why.
```

## Shops, members and courses

### mailpoet

MailPoet: subscribers (email, names, IPs), sending queues truncated, statistics kept as counts.

```text
MailPoet (mailpoet)
checked against: docs: https://github.com/mailpoet/mailpoet/blob/trunk/mailpoet/lib/Entities/SubscriberEntity.php
detected by: a table of {prefix}{site}mailpoet_subscribers; or a file of wp-content/plugins/mailpoet/mailpoet.php

  TABLE                                           COLUMN            RULE
  {prefix}{site}mailpoet_subscribers              email             fake email
  {prefix}{site}mailpoet_subscribers              first_name        fake first_name
  {prefix}{site}mailpoet_subscribers              last_name         fake last_name
  {prefix}{site}mailpoet_subscribers              subscribed_ip     fake ip
  {prefix}{site}mailpoet_subscribers              confirmed_ip      fake ip
  {prefix}{site}mailpoet_subscribers              unconfirmed_data  scrub text
  {prefix}{site}mailpoet_subscriber_custom_field  value             scrub text
  {prefix}{site}mailpoet_subscriber_ips           ip                fake ip

truncated: {prefix}{site}mailpoet_sending_queues, {prefix}{site}mailpoet_log

kept: {prefix}{site}mailpoet_statistics_*

rhythm-roadie explain mailpoet says why.
```

### edd

Easy Digital Downloads: customers, addresses, customer emails, orders and order addresses; logs scrubbed.

```text
Easy Digital Downloads (edd)
checked against: docs: https://github.com/awesomemotive/easy-digital-downloads/tree/main/includes/database/schemas
detected by: a table of {prefix}{site}edd_customers, {prefix}{site}edd_orders; or a file of wp-content/plugins/easy-digital-downloads/easy-digital-downloads.php

  TABLE                                       COLUMN                                    RULE
  {prefix}{site}edd_customers                 email                                     fake email
  {prefix}{site}edd_customers                 name                                      fake full_name
  {prefix}{site}edd_customermeta              meta_value (meta_key = additional_email)  fake email
  {prefix}{site}edd_customer_email_addresses  email                                     fake email
  {prefix}{site}edd_customer_addresses        name                                      fake full_name
  {prefix}{site}edd_customer_addresses        address                                   fake street
  {prefix}{site}edd_customer_addresses        address2                                  fake street
  {prefix}{site}edd_customer_addresses        city                                      fake city
  {prefix}{site}edd_customer_addresses        region                                    kept (keep: state)
  {prefix}{site}edd_customer_addresses        postal_code                               fake postcode
  {prefix}{site}edd_customer_addresses        country                                   kept (keep: country)
  {prefix}{site}edd_orders                    email                                     fake email
  {prefix}{site}edd_orders                    ip                                        fake ip
  {prefix}{site}edd_order_addresses           name                                      fake full_name
  {prefix}{site}edd_order_addresses           address                                   fake street
  {prefix}{site}edd_order_addresses           address2                                  fake street
  {prefix}{site}edd_order_addresses           city                                      fake city
  {prefix}{site}edd_order_addresses           region                                    kept (keep: state)
  {prefix}{site}edd_order_addresses           postal_code                               fake postcode
  {prefix}{site}edd_order_addresses           country                                   kept (keep: country)
  {prefix}{site}edd_order_transactions        transaction_id                            blank
  {prefix}{site}edd_logs                      title                                     scrub text
  {prefix}{site}edd_logs                      content                                   scrub text
  {prefix}{site}edd_logs_file_downloads       ip                                        fake ip
  {prefix}{site}edd_logs_file_downloads       user_agent                                fake user_agent
  {prefix}{site}edd_notes                     content                                   scrub text
  {prefix}{site}edd_logs_emails               email                                     fake email
  {prefix}{site}edd_logs_emails               subject                                   scrub text

truncated: {prefix}{site}edd_logs_api_requests

rhythm-roadie explain edd says why.
```

### memberpress

MemberPress: members, transaction IPs, subscription gateway IDs blanked and mepr-address usermeta.

```text
MemberPress (memberpress)
checked against: docs: https://memberpress.com/docs/where-is-data-stored/
detected by: a table of {prefix}{site}mepr_transactions, {prefix}{site}mepr_members; or a file of wp-content/plugins/memberpress/memberpress.php

  TABLE                             COLUMN                                        RULE
  {prefix}{site}mepr_transactions   ip_addr                                       fake ip
  {prefix}{site}mepr_transactions   trans_num                                     blank
  {prefix}{site}mepr_transactions   response                                      blank
  {prefix}{site}mepr_subscriptions  subscr_id                                     blank
  {prefix}{site}mepr_subscriptions  ip_addr                                       fake ip
  {prefix}{site}mepr_subscriptions  token                                         blank
  {prefix}{site}mepr_subscriptions  cc_last4                                      blank
  {prefix}{site}mepr_subscriptions  response                                      blank
  {prefix}usermeta                  meta_value (meta_key = mepr-address-one)      fake street
  {prefix}usermeta                  meta_value (meta_key = mepr-address-two)      fake street
  {prefix}usermeta                  meta_value (meta_key = mepr-address-city)     fake city
  {prefix}usermeta                  meta_value (meta_key = mepr-address-zip)      fake postcode
  {prefix}usermeta                  meta_value (meta_key = mepr-address-state)    kept (keep: state)
  {prefix}usermeta                  meta_value (meta_key = mepr-address-country)  kept (keep: country)

kept: {prefix}{site}mepr_members

rhythm-roadie explain memberpress says why.
```

### learndash

LearnDash: quiz toplist (email, name, IP); user activity kept.

```text
LearnDash (learndash)
checked against: docs: https://plugins.svn.wordpress.org/wp-pro-quiz/trunk/lib/helper/WpProQuiz_Helper_DbUpgrade.php
detected by: a table of {prefix}{site}learndash_user_activity, {prefix}{site}learndash_pro_quiz_toplist; or a file of wp-content/plugins/sfwd-lms/sfwd_lms.php

  TABLE                                           COLUMN     RULE
  {prefix}{site}learndash_pro_quiz_toplist        name       fake full_name
  {prefix}{site}learndash_pro_quiz_toplist        email      fake email
  {prefix}{site}learndash_pro_quiz_toplist        ip         fake ip
  {prefix}{site}learndash_pro_quiz_statistic_ref  form_data  scrub text

kept: {prefix}{site}learndash_user_activity

rhythm-roadie explain learndash says why.
```

## Laravel

### laravel

Laravel, from laravel.tables: users (name, email, password, remember and two-factor tokens), sessions, tokens, cache, jobs and telescope tables truncated, notifications scrubbed; .env set for staging and Echo.

```text
Laravel (laravel)
checked against: docs: https://github.com/laravel/laravel/blob/12.x/database/migrations/0001_01_01_000000_create_users_table.php
detected by: a table of migrations

  TABLE          COLUMN                     RULE
  users          name                       fake full_name
  users          email                      fake email
  users          password                   reset-password
  users          remember_token             null
  users          two_factor_secret          null
  users          two_factor_recovery_codes  null
  notifications  data                       scrub text

truncated: password_reset_tokens, password_resets, sessions, personal_access_tokens, cache, cache_locks, jobs, job_batches, failed_jobs, telescope_entries, telescope_entries_tags, telescope_monitoring, pulse_*

rhythm-roadie explain laravel says why.
```

### laravel-cashier

Laravel Cashier: stripe_id and card details nulled; subscriptions and items kept.

```text
Laravel Cashier (laravel-cashier)
checked against: docs: https://github.com/laravel/cashier-stripe/tree/15.x/database/migrations
detected by: a table of subscription_items

  TABLE          COLUMN         RULE
  users          stripe_id      null
  users          pm_type        null
  users          pm_last_four   null
  users          trial_ends_at  keep
  subscriptions  stripe_id      keep
  subscriptions  stripe_price   keep

kept: subscription_items

rhythm-roadie explain laravel-cashier says why.
```

## Your own schema

### sql

Generic SQL: only what columns: lists, plus the uncovered-column scan by name and content.

```text
Generic SQL (sql)
checked against: config: the tables columns: lists

rhythm-roadie explain sql says why.
```

---
<sub>← [Sources and targets](Roadie-Sources-And-Targets) · [Home](Home) · [Pseudonymisation](Roadie-Pseudonymisation) →</sub>
