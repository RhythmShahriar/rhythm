// Copyright (c) 2026 Rhythm Shahriar <me@rhy.io>
// SPDX-License-Identifier: AGPL-3.0-only

package secrets

import (
	"regexp"
	"strings"
)

// token is a rule for a provider format whose whole match, capture group
// 1, is the secret. hint is the lower-case literal the format starts with.
func token(id, desc, tags, hint, pattern string, keep int) secretRule {
	return secretRule{id: id, desc: desc, tags: strings.Fields(tags), hint: strings.Split(hint, "|"),
		re: regexp.MustCompile(pattern), value: 1, keep: keep}
}

// nameHints are the literals a secret-looking name contains (see
// isSecretName), the hint for the generic name = value rules.
var nameHints = []string{"pass", "pw", "secret", "token", "key", "auth", "dsn", "credential", "otp"}

// Shared pieces of the generic name = value rules.
const (
	// A double-quoted value with backslash escapes, a single-quoted one,
	// or a bare one (the last alternative is added by each rule).
	dq = `"((?:[^"\\\r\n]|\\.)*)"`
	sq = `'((?:[^'\\\r\n]|\\.)*)'`
	// scheme is an HTTP auth scheme word written before a token, which
	// rules skip so the token itself is masked.
	scheme = `(?i:bearer|token|basic|digest)`
)

// secretRules is the catalog, applied in order. Specific formats come
// first: when two rules match the same text, the earlier one wins, so a
// Stripe key in an env file is reported as a Stripe key. Provider
// formats follow each provider's published token prefixes and lengths.
var secretRules = []secretRule{
	// Cloud and infrastructure.
	token("aws-access-key", "AWS access key ID", "cloud aws", "akia|asia|abia|acca", `\b((?:AKIA|ASIA|ABIA|ACCA)[0-9A-Z]{16})\b`, 4),
	{id: "aws-secret-key", desc: "AWS secret access key next to its name", tags: []string{"cloud", "aws"}, hint: []string{"aws"},
		re: regexp.MustCompile(`(?i)aws[\w.-]{0,20}?secret[\w.-]{0,20}?["']?\s*[:=]+\s*["']?([A-Za-z0-9/+]{40})\b`), value: 1, keep: 0},
	token("google-api-key", "Google API key", "cloud google", "aiza", `\b(AIza[0-9A-Za-z_\-]{35})`, 4),
	token("google-oauth-secret", "Google OAuth client secret", "cloud google auth", "gocspx-", `\b(GOCSPX-[A-Za-z0-9_\-]{28})`, 4),
	{id: "azure-storage-key", desc: "Azure storage account key in a connection string", tags: []string{"cloud", "azure"}, hint: []string{"accountkey="},
		re: regexp.MustCompile(`(?i)AccountKey=([A-Za-z0-9+/]{86}==)`), value: 1},
	token("azure-client-secret", "Azure AD client secret", "cloud azure auth", "q~", `\b([A-Za-z0-9_~.]{3}\dQ~[A-Za-z0-9_~.\-]{31,34})`, 0),
	token("digitalocean-token", "DigitalOcean API, OAuth or refresh token", "cloud", "_v1_", `\b(do[opr]_v1_[a-f0-9]{64})\b`, 4),
	token("heroku-api-key", "Heroku API key", "cloud", "hrku-", `\b(HRKU-[A-Za-z0-9_\-]{60,})`, 4),
	token("alibaba-access-key", "Alibaba Cloud access key ID", "cloud", "ltai", `\b(LTAI[A-Za-z0-9]{20})\b`, 4),
	token("cloudflare-origin-ca-key", "Cloudflare Origin CA key", "cloud", "v1.0-", `\b(v1\.0-[a-f0-9]{24}-[a-f0-9]{146})\b`, 4),
	token("netlify-token", "Netlify personal access token", "cloud", "nfp_", `\b(nfp_[A-Za-z0-9]{36,})`, 4),
	token("vault-token", "HashiCorp Vault token", "cloud", "hvs.|hvb.|hvr.", `\b(hv[sbr]\.[A-Za-z0-9_\-]{24,})`, 4),
	token("terraform-cloud-token", "Terraform Cloud API token", "cloud ci", ".atlasv1.", `\b([a-z0-9]{14}\.atlasv1\.[A-Za-z0-9_\-]{60,})`, 0),
	token("doppler-token", "Doppler token", "cloud", "dp.", `\b(dp\.(?:pt|st|sa|ct|scim|audit)\.[A-Za-z0-9]{40,})`, 5),
	token("pulumi-token", "Pulumi access token", "cloud ci", "pul-", `\b(pul-[a-f0-9]{40})\b`, 4),
	token("databricks-token", "Databricks access token", "cloud database", "dapi", `\b(dapi[a-f0-9]{32}(?:-\d)?)\b`, 4),
	token("supabase-token", "Supabase access token or secret key", "cloud database", "sbp_|sb_secret_", `\b(sbp_[a-f0-9]{40}|sb_secret_[A-Za-z0-9_\-]{20,})`, 4),
	token("planetscale-token", "PlanetScale token or password", "cloud database", "pscale_", `\b(pscale_(?:tkn|pw|oauth)_[A-Za-z0-9_.\-]{32,})`, 4),
	token("age-secret-key", "age encryption secret key", "crypto", "age-secret-key-", `\b(AGE-SECRET-KEY-1[QPZRY9X8GF2TVDW0S3JN54KHCE6MUA7L]{58})\b`, 4),
	token("onepassword-service-token", "1Password service account token", "cloud", "ops_ey", `\b(ops_eyJ[A-Za-z0-9+/=_\-]{100,})`, 4),

	// Payment and commerce.
	token("stripe-key", "Stripe secret or restricted key", "payment", "sk_|rk_", `\b((?:sk|rk)_(?:live|test)_[0-9a-zA-Z]{16,})`, 8),
	token("stripe-webhook-secret", "Stripe webhook signing secret", "payment", "whsec_", `\b(whsec_[0-9a-zA-Z]{16,})`, 6),
	token("square-token", "Square access token or OAuth secret", "payment", "eaaa|sq0", `\b(EAAA[A-Za-z0-9_\-]{60}|sq0(?:csp|atp)-[0-9A-Za-z_\-]{22,43})`, 4),
	token("braintree-token", "Braintree or PayPal access token", "payment", "access_token$", `\b(access_token\$(?:production|sandbox)\$[0-9a-z]{16}\$[0-9a-f]{32})\b`, 4),
	token("shopify-token", "Shopify access token or shared secret", "payment commerce", "shpat_|shpca_|shppa_|shpss_", `\b(shp(?:at|ca|pa|ss)_[a-fA-F0-9]{32})\b`, 4),
	token("flutterwave-key", "Flutterwave secret key", "payment", "flwseck", `\b(FLWSECK(?:_TEST)?-[a-hA-H0-9]{32}-X)\b`, 4),
	token("easypost-key", "EasyPost API key", "commerce", "ezak|ezte", `\b(EZ[AT]K[a-zA-Z0-9]{54})\b`, 4),
	token("shippo-token", "Shippo API token", "commerce", "shippo_", `\b(shippo_(?:live|test)_[a-f0-9]{40})\b`, 4),
	token("duffel-token", "Duffel API token", "commerce", "duffel_", `\b(duffel_(?:test|live)_[A-Za-z0-9_\-]{43})`, 4),

	// Mail and messaging.
	token("slack-token", "Slack bot, user or workspace token", "messaging", "xox", `\b(xox[abposr]-[0-9A-Za-z-]{10,})`, 5),
	token("slack-app-token", "Slack app-level token", "messaging", "xapp-", `\b(xapp-\d-[A-Z0-9]+-\d+-[a-z0-9]+)`, 4),
	{id: "slack-webhook", desc: "Slack incoming webhook URL", tags: []string{"messaging"}, hint: []string{"hooks.slack.com"},
		re: regexp.MustCompile(`hooks\.slack\.com/(?:services|workflows|triggers)/[A-Z0-9]+/[A-Z0-9]+/([0-9A-Za-z]{8,})`), value: 1, keep: 2},
	{id: "discord-webhook", desc: "Discord webhook URL", tags: []string{"messaging"}, hint: []string{"/api/webhooks/"},
		re: regexp.MustCompile(`discord(?:app)?\.com/api/webhooks/\d+/([0-9A-Za-z_\-]{20,})`), value: 1, keep: 2},
	token("discord-bot-token", "Discord bot token", "messaging", ".", `\b([MN][A-Za-z\d]{23,25}\.[\w-]{6}\.[\w-]{27,38})\b`, 0),
	{id: "teams-webhook", desc: "Microsoft Teams incoming webhook URL", tags: []string{"messaging"}, hint: []string{"webhook.office.com"},
		re: regexp.MustCompile(`\.webhook\.office\.com/webhookb2/([^\s"'<>]{20,})`), value: 1, keep: 2},
	{id: "telegram-bot-token", desc: "Telegram bot token, bare or in an API URL", tags: []string{"messaging"}, hint: []string{":aa"},
		re: regexp.MustCompile(`(?:\b|bot)(\d{8,10}:AA[0-9A-Za-z_\-]{33})\b`), value: 1, keep: 0},
	token("twilio-api-key", "Twilio API key SID", "messaging", "sk", `\b(SK[0-9a-f]{32})\b`, 2),
	{id: "twilio-auth-token", desc: "Twilio auth token next to its account SID or its name", tags: []string{"messaging"}, hint: []string{"ac", "twilio"},
		re: regexp.MustCompile(`(?i:\bAC[0-9a-f]{32}\b[^\r\n]{0,80}?|twilio[\w .-]{0,20}?(?:auth[\w .-]{0,10}?token|secret)["']?\s*[:=]\s*["']?)\b([0-9a-f]{32})\b`), value: 1},
	token("sendgrid-key", "SendGrid API key", "mail", "sg.", `\b(SG\.[0-9A-Za-z_\-]{16,}\.[0-9A-Za-z_\-]{16,})`, 3),
	token("mailgun-key", "Mailgun API key", "mail", "key-", `\b(key-[0-9a-f]{32})\b`, 4),
	token("mailchimp-key", "Mailchimp API key", "mail", "-us", `\b([0-9a-f]{32}-us\d{1,2})\b`, 0),
	token("brevo-key", "Brevo (Sendinblue) API key", "mail", "xkeysib-|xsmtpsib-", `\b(x(?:key|smtp)sib-[a-f0-9]{64}-[A-Za-z0-9]{16})\b`, 4),
	token("resend-key", "Resend API key", "mail", "re_", `\b(re_[0-9A-Za-z_]{20,})`, 3),

	// Source control, packages and CI.
	token("github-token", "GitHub token", "vcs", "ghp_|gho_|ghu_|ghs_|ghr_", `\b(gh[pousr]_[A-Za-z0-9]{36,})`, 4),
	token("github-fine-grained-token", "GitHub fine-grained personal access token", "vcs", "github_pat_", `\b(github_pat_[0-9A-Za-z_]{82})\b`, 4),
	token("gitlab-token", "GitLab personal, deploy, runner or trigger token", "vcs ci", "gl", `\b(gl(?:pat|dt|ptt|rt|soat|ft|cbt|imt)-[0-9A-Za-z_\-]{20,})`, 4),
	token("atlassian-token", "Atlassian API token", "vcs", "atatt3", `\b(ATATT3[A-Za-z0-9_=\-]{100,})`, 4),
	token("npm-token", "npm access token", "vcs packages", "npm_", `\b(npm_[A-Za-z0-9]{36})\b`, 4),
	token("pypi-token", "PyPI upload token", "vcs packages", "pypi-", `\b(pypi-AgEIcHlwaS5vcmc[A-Za-z0-9_\-]{50,})`, 4),
	token("rubygems-key", "RubyGems API key", "vcs packages", "rubygems_", `\b(rubygems_[a-f0-9]{48})\b`, 4),
	token("dockerhub-token", "Docker Hub personal access token", "vcs packages", "dckr_pat_", `\b(dckr_pat_[A-Za-z0-9_\-]{27,})`, 4),
	token("nuget-key", "NuGet API key", "vcs packages", "oy2", `\b(oy2[a-z0-9]{43})\b`, 3),
	token("circleci-token", "CircleCI personal API token", "ci", "ccipat_", `\b(CCIPAT_[A-Za-z0-9_]{20,})`, 4),
	token("buildkite-token", "Buildkite API token", "ci", "bkua_", `\b(bkua_[a-f0-9]{40})\b`, 4),
	token("sonarqube-token", "SonarQube token", "ci", "squ_|sqp_|sqa_", `\b(sq[upa]_[a-f0-9]{40})\b`, 4),
	token("jfrog-key", "JFrog API key", "ci packages", "akcp", `\b(AKCp[A-Za-z0-9]{69})\b`, 4),

	// Monitoring and SaaS.
	token("grafana-token", "Grafana Cloud or service account token", "monitoring", "glc_|glsa_", `\b(glc_[A-Za-z0-9+/]{32,}={0,2}|glsa_[A-Za-z0-9]{32}_[A-Fa-f0-9]{8})`, 4),
	token("sentry-token", "Sentry auth token", "monitoring", "sntrys_|sntryu_", `\b(sntrys_eyJ[A-Za-z0-9+/=_\-]{50,}|sntryu_[a-f0-9]{64})`, 4),
	token("newrelic-key", "New Relic user API key", "monitoring", "nrak-", `\b(NRAK-[A-Z0-9]{27})\b`, 4),
	token("linear-key", "Linear API key", "saas", "lin_api_", `\b(lin_api_[A-Za-z0-9]{40})\b`, 4),
	token("notion-token", "Notion integration token", "saas", "ntn_", `\b(ntn_[0-9]{11}[A-Za-z0-9]{35})\b`, 4),
	token("figma-token", "Figma personal access token", "saas", "figd_", `\b(figd_[A-Za-z0-9_\-]{40,})`, 4),
	token("airtable-token", "Airtable personal access token", "saas", "pat", `\b(pat[A-Za-z0-9]{14}\.[a-f0-9]{64})\b`, 3),
	token("hubspot-token", "HubSpot private app token", "saas", "pat-na1-|pat-eu1-", `\b(pat-(?:na|eu)1-[a-f0-9]{8}-[a-f0-9]{4}-[a-f0-9]{4}-[a-f0-9]{4}-[a-f0-9]{12})\b`, 4),
	token("postman-key", "Postman API key", "saas", "pmak-", `\b(PMAK-[a-f0-9]{24}-[a-f0-9]{34})\b`, 4),
	token("contentful-token", "Contentful personal access token", "saas", "cfpat-", `\b(CFPAT-[A-Za-z0-9_\-]{43})`, 4),
	token("typeform-token", "Typeform personal access token", "saas", "tfp_", `\b(tfp_[A-Za-z0-9_.=\-]{40,})`, 4),
	token("dropbox-token", "Dropbox access token", "saas", "sl.", `\b(sl\.[A-Za-z0-9_\-]{100,})`, 3),
	token("mapbox-token", "Mapbox secret token", "saas", "sk.ey", `\b(sk\.eyJ[A-Za-z0-9_\-]{50,}\.[A-Za-z0-9_\-]{20,})`, 3),

	// AI providers. Anthropic and OpenRouter come before OpenAI because
	// all three start with sk-.
	token("anthropic-key", "Anthropic API key", "ai", "sk-ant-", `\b(sk-ant-[A-Za-z0-9_\-]{20,})`, 4),
	token("openrouter-key", "OpenRouter API key", "ai", "sk-or-", `\b(sk-or-v1-[a-f0-9]{64})\b`, 4),
	token("openai-key", "OpenAI API key", "ai", "sk-", `\b(sk-(?:proj|svcacct|admin)-[A-Za-z0-9_\-]{20,}|sk-[A-Za-z0-9]{20}T3BlbkFJ[A-Za-z0-9]{20})`, 4),
	token("huggingface-token", "Hugging Face access token", "ai", "hf_", `\b(hf_[A-Za-z]{34})\b`, 3),
	token("replicate-token", "Replicate API token", "ai", "r8_", `\b(r8_[A-Za-z0-9]{37})\b`, 3),
	token("groq-key", "Groq API key", "ai", "gsk_", `\b(gsk_[A-Za-z0-9]{52})\b`, 4),
	token("perplexity-key", "Perplexity API key", "ai", "pplx-", `\b(pplx-[A-Za-z0-9]{48})\b`, 4),
	token("xai-key", "xAI API key", "ai", "xai-", `\b(xai-[A-Za-z0-9]{80})\b`, 4),

	// Shapes that are not tied to one provider.
	{id: "private-key", desc: "PEM private key block, encrypted or not", tags: []string{"crypto"}, hint: []string{"private key"},
		re: regexp.MustCompile(`-----BEGIN ([A-Z0-9 ]*PRIVATE KEY(?: BLOCK)?)-----((?s:.+?))(?:-----END [A-Z0-9 ]*PRIVATE KEY(?: BLOCK)?-----|\z)`), name: 1, value: 2, multiline: true},
	token("jwt", "JSON Web Token", "auth web", "eyj", `\b(eyJ[A-Za-z0-9_\-]{5,}\.eyJ[A-Za-z0-9_\-]{5,}\.[A-Za-z0-9_\-]{10,})`, 3),
	{id: "url-credentials", desc: "Password in a URL (scheme://user:password@host)", tags: []string{"url", "database"}, hint: []string{"://"},
		// The password runs to the last @ before the host, since
		// generated passwords often contain / and @.
		re: regexp.MustCompile(`\b[a-zA-Z][a-zA-Z0-9+.\-]*://[^:\s/@"'<>]*:([^\s"'<>?#]+)@[^\s/@?#"'<>]+`), value: 1, keep: 2,
		valid: notPort, anyLength: true},
	{id: "authorization", desc: "HTTP Authorization header, whatever the scheme", tags: []string{"auth", "web"}, hint: []string{"authorization"},
		re: regexp.MustCompile(`(?i)\b((?:proxy-)?authorization)["']?[ \t]*:[ \t]*(?:"([^"\r\n]*)"|'([^'\r\n]*)'|(\S(?:[^\r\n]*\S)?))`), name: 1, value: 2},
	{id: "cookie", desc: "Cookie and Set-Cookie values (names stay readable)", tags: []string{"auth", "web"}, hint: []string{"cookie"},
		re:   regexp.MustCompile(`(?i)\b((?:set-)?cookie)["']?[ \t]*:[ \t]*(?:"([^"\r\n]*)"|(\S(?:[^\r\n]*\S)?))`),
		name: 1, value: 2, keep: 0, split: cookieValues},
	{id: "header-token", desc: "API key and token headers (X-Auth-Token, X-Api-Key, Private-Token…)", tags: []string{"auth", "web"}, hint: []string{"x-", "api-key", "apikey", "private-token"},
		// An optional scheme word (X-Api-Key: Bearer abc...) stays readable
		// and the token after it is masked.
		re:   regexp.MustCompile(`(?i)\b(x-[a-z0-9-]*(?:token|key|secret|auth|nonce|signature|password)[a-z0-9-]*|api-?key|private-token)["']?[ \t]*[:=][ \t]*["']?(?:` + scheme + `[ \t]+)?([^\s"',;]{6,})`),
		name: 1, value: 2},
	{id: "wp-reset-key", desc: "Password reset key or one-time code in a link (key=, k=, code=, otp= next to action=rp or login=)", tags: []string{"wordpress", "auth", "url"}, hint: []string{"key=", "k=", "code=", "otp="},
		re: regexp.MustCompile(`(?i)[?&;](?:amp;)?(key|k|code|otp)=([^&\s"'<>#]+)`), value: 2, valid: resetQuery},
	{id: "oauth-code", desc: "OAuth authorization code in a callback link (code= next to state=)", tags: []string{"auth", "url"}, hint: []string{"code="},
		re: regexp.MustCompile(`(?i)[?&;](?:amp;)?code=([^&\s"'<>#]+)`), value: 1, valid: oauthQuery},
	{id: "key-path-token", desc: "Token in a /key/ or /token/ path segment", tags: []string{"auth", "url"}, hint: []string{"/key/", "/token/", "/apikey/", "/api-key/"},
		re:    regexp.MustCompile(`(?i)/(?:key|token|api-?key)/([A-Za-z0-9_\-.%=]{12,})`),
		value: 1, keep: 0, valid: hasDigitAndLetter},
	{id: "reset-link-token", desc: "Token in a password reset, verification or magic link path", tags: []string{"auth", "url"}, hint: []string{"reset", "verify", "confirm", "magic", "login"},
		re:    regexp.MustCompile(`(?i)/[a-z0-9_\-]*(?:reset|verify|confirm|magic|login)[a-z0-9_\-]*(?:/[^/\s?#"'<>]{1,12})?/([A-Za-z0-9_\-.%=]{16,})`),
		value: 1, keep: 0, valid: hasDigitAndLetter},
	{id: "wp-config", desc: "wp-config.php database password, keys and salts", tags: []string{"wordpress", "config"}, hint: []string{"define"},
		re: regexp.MustCompile(`define\(\s*['"](DB_USER|[A-Z0-9_]*(?:PASS|PASSWORD|KEY|SALT|SECRET|TOKEN)[A-Z0-9_]*)['"]\s*,\s*['"]([^'"]+)['"]`), name: 1, value: 2, keep: 2},
	{id: "php-serialized", desc: "Secret option in PHP serialized data (SMTP, backup and S3 plugin settings)", tags: []string{"wordpress", "config"}, hint: []string{"s:"},
		re: regexp.MustCompile(`s:\d+:"([^"]{1,64})";s:\d+:"([^"]*)"`), name: 1, value: 2, keep: 2, secretName: true},
	{id: "npmrc-auth", desc: "Registry credentials in .npmrc", tags: []string{"config", "packages"}, hint: []string{":_auth", ":_password"},
		re: regexp.MustCompile(`:(_authToken|_auth|_password)[ \t]*=[ \t]*(\S+)`), name: 1, value: 2, keep: 2},
	{id: "curl-user", desc: "Password in curl -u user:password", tags: []string{"auth"}, hint: []string{"-u", "--user"},
		re: regexp.MustCompile(`(?:\s-u|--user)[ =]["']?[^:\s"']+:([^\s"']+)`), value: 1, keep: 2, anyLength: true},

	// Secret-looking names. These run last, so a known format inside the
	// value keeps its own kind.
	{id: "env", desc: "Secret-looking variable in an env file or shell export", tags: []string{"config"}, hint: nameHints,
		re:   regexp.MustCompile(`(?m)^[ \t]*(?:export[ \t]+)?([A-Za-z_][A-Za-z0-9_.]*)[ \t]*=[ \t]*(?:` + dq + `|` + sq + `|(?:` + scheme + `[ \t]+)?(\S+))`),
		name: 1, value: 2, keep: 2, secretName: true, valid: notQuery},
	{id: "yaml", desc: "Secret-looking key in YAML", tags: []string{"config"}, hint: nameHints,
		re:   regexp.MustCompile(`(?m)^[ \t]*(?:-[ \t]+)?([A-Za-z_][A-Za-z0-9_.\-]*)[ \t]*:[ \t]+(?:` + dq + `|` + sq + `|([^\s"'#&*!|>{\[][^\r\n]*?))[ \t]*(?:[ \t]#[^\r\n]*)?\r?$`),
		name: 1, value: 2, keep: 2, secretName: true},
	{id: "json-field", desc: "Secret-looking field in JSON", tags: []string{"config", "web"}, hint: nameHints,
		re:   regexp.MustCompile(`"([^"\\\r\n]{1,64})"\s*:\s*(?:"((?:[^"\\]|\\.)+)"|(-?\d{4,}))`),
		name: 1, value: 2, keep: 2, secretName: true},
	{id: "assignment", desc: "Secret-looking variable assigned a quoted string in code", tags: []string{"config"}, hint: nameHints,
		re:   regexp.MustCompile(`(?:^|[^A-Za-z0-9_$])\$?([A-Za-z_][A-Za-z0-9_.]*)["']?\]?[ \t]*(?:=>|:=|[:=])[ \t]*(?:` + dq + `|` + sq + `)`),
		name: 1, value: 2, keep: 2, secretName: true},
	{id: "form-field", desc: "Secret-looking parameter in a query string or form body (pwd, pass1, password…)", tags: []string{"web"}, hint: nameHints,
		re:   regexp.MustCompile(`(?:^|[?&;\s])([A-Za-z0-9_.\-\[\]]+)=(?:` + scheme + `(?:\+|%20|[ \t]+))?([^&\s"'<>;]+)`),
		name: 1, value: 2, keep: 2, secretName: true},
}

// queryPair finds a second name=value pair, the sign of a form body.
var queryPair = regexp.MustCompile(`&[A-Za-z0-9_.\-\[\]]+=`)

// notQuery leaves a bare env value that is really a form body
// (pass1=a&pass2=b) to the form-field rule, which masks each pair.
func notQuery(s string, m []int) bool { return m[8] < 0 || !queryPair.MatchString(s[m[8]:m[9]]) }

// portPath matches a URL whose "password" is really a port and a path:
// https://host:8080/@alice.
var portPath = regexp.MustCompile(`^\d{1,5}/`)

func notPort(s string, m []int) bool { return !portPath.MatchString(s[m[2]:m[3]]) }

// resetQuery accepts a key=, k=, code= or otp= value only when its query
// string also has action=rp, action=resetpass or login=, the shape of a
// WordPress password reset link. Elsewhere these are too often ordinary
// parameters (a map key, a coupon code).
func resetQuery(s string, m []int) bool {
	return queryHas(s, m, "action=rp", "action=resetpass", "login=")
}

// oauthQuery accepts a code= value when its query string also has state=,
// the shape of an OAuth callback.
func oauthQuery(s string, m []int) bool { return queryHas(s, m, "state=") }

// queryHas reports whether the query string around match m has one of
// the name=value pairs in want ("&amp;" counts as "&").
func queryHas(s string, m []int, want ...string) bool {
	start := strings.LastIndexAny(s[:m[0]+1], "? \t\r\n\"'<>")
	if start < 0 || s[start] != '?' {
		return false
	}
	end := m[1] + strings.IndexAny(s[m[1]:]+" ", " \t\r\n\"'<>#")
	q := strings.ToLower(strings.ReplaceAll(s[start:end], "&amp;", "&"))
	for _, w := range want {
		if strings.Contains(q, "?"+w) || strings.Contains(q, "&"+w) || strings.Contains(q, ";"+w) {
			return true
		}
	}
	return false
}

// hasDigitAndLetter keeps path tokens that look generated: words such as
// /confirm/order-summary-page are left alone.
func hasDigitAndLetter(s string, m []int) bool {
	v := s[m[2]:m[3]]
	return strings.ContainsAny(v, "0123456789") && strings.ContainsFunc(v, func(r rune) bool { return r >= 'A' && r <= 'Z' || r >= 'a' && r <= 'z' })
}

// Set-Cookie attributes, which describe a cookie rather than hold one.
var cookieAttrs = map[string]bool{"path": true, "domain": true, "expires": true, "max-age": true, "samesite": true, "secure": true, "httponly": true, "priority": true, "partitioned": true, "version": true, "comment": true}

// cookieValues finds every name=value pair in a Cookie or Set-Cookie
// value and returns the values, so the names stay readable.
func cookieValues(v string) []part {
	var out []part
	at := 0
	for _, pair := range strings.Split(v, ";") {
		name, val, ok := strings.Cut(pair, "=")
		name = strings.TrimSpace(name)
		if ok && name != "" && !cookieAttrs[strings.ToLower(name)] {
			start := at + len(pair) - len(val) // just past the =
			trimmed := strings.TrimSpace(val)
			start += strings.Index(val, trimmed)
			out = append(out, part{name: name, start: start, end: start + len(trimmed)})
		}
		at += len(pair) + 1
	}
	return out
}
