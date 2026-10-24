// Copyright (c) 2026 Rhythm Shahriar <me@rhy.io>
// SPDX-License-Identifier: AGPL-3.0-only

package secrets

import (
	"slices"
	"strings"
	"testing"
)

// Provider-format fixtures are assembled at run time, so the source never
// contains anything a secret scanner would flag. All values are fake.
func j(parts ...string) string { return strings.Join(parts, "") }

// cycle repeats alphabet to n characters.
func cycle(alphabet string, n int) string {
	var b strings.Builder
	for i := range n {
		b.WriteByte(alphabet[i%len(alphabet)])
	}
	return b.String()
}

func mix(n int) string     { return cycle("aB3dE5gH7jK9mN2pQ4sT6vW8yZ1", n) }
func hex(n int) string     { return cycle("0123456789abcdef", n) }
func digits(n int) string  { return cycle("1234567890", n) }
func letters(n int) string { return cycle("abCDefGHijKLmnOPqrSTuvWXyz", n) }

func TestFindSecretsMasks(t *testing.T) {
	raw := map[string]string{
		"DB_PASSWORD=supersecret\n":                                     "supersecret",
		"STRIPE_KEY=sk_live_abcdefghijklmnop1234\n":                     "sk_live_abcdefghijklmnop1234",
		"key AKIAABCDEFGHIJKLMNOP here":                                 "AKIAABCDEFGHIJKLMNOP",
		"define( 'DB_PASSWORD', 'wp-pass-123' );":                       "wp-pass-123",
		"DATABASE_URL=mysql://root:hunter22@db:3306/app":                "hunter22",
		"-----BEGIN RSA PRIVATE KEY-----\nMIIEowIBAAKCAQEAx1234567\n":   "MIIEowIBAAKCAQEAx1234567",
		"Authorization: Bearer re_live_abcdefghijklmnopqrstu":           "abcdefghijklmnopqrstu",
		"Authorization: Basic QUMxMjM6c2VjcmV0dG9rZW4=":                 "QUMxMjM6c2VjcmV0dG9rZW4=",
		`{"api_key":"abc123def456ghi","to":"+15550001"}`:                "abc123def456ghi",
		"From=%2B1555&Body=hi&access_token=EAAGm0PX4ZCpsBAF0q":          "EAAGm0PX4ZCpsBAF0q",
		"x-postmark-server-token: 1b2c3d4e-0000-4f00-8000-123456789abc": "1b2c3d4e-0000-4f00-8000-123456789abc",
	}
	for in, secret := range map[string]string{
		j("https://hooks.slack.com/services/T0001/B0002/", "XXXXabcd", "YYYYefgh"): "XXXXabcdYYYYefgh",
		j("whs", "ec_", "1234567890abcdefghijklmn"):                                "1234567890abcdefghijklmn",
		j("S", "G.", strings.Repeat("a", 22), ".", strings.Repeat("B", 43)):        strings.Repeat("B", 43),
		j("ke", "y-", strings.Repeat("0f", 16)):                                    strings.Repeat("0f", 16),
	} {
		raw[in] = secret
	}
	for in, secret := range raw {
		got := FindSecrets([]byte(in))
		if len(got) == 0 {
			t.Errorf("%q: no secret found", in)
			continue
		}
		for _, s := range got {
			if strings.Contains(s.String(), secret) {
				t.Errorf("%q: masked output %q leaks the secret", in, s)
			}
		}
		if red := Redact(in); strings.Contains(red, secret) {
			t.Errorf("Redact(%q) = %q leaks the secret", in, red)
		}
	}
}

// TestRedactShapes covers the shapes found missing in the October 2026
// review: each fixture names the parts that must be gone and the parts
// (names, structure) that must stay.
func TestRedactShapes(t *testing.T) {
	jwt := "eyJhbGciOiJIUzI1NiJ9.eyJzdWIiOiIxMjM0In0.dozjgNryP4J3jVmNHl0w5N_XgL0n3I9PlFUP0THsR8U"
	for _, c := range []struct {
		in         string
		gone, kept []string
	}{
		// URL credentials with / and @ in the password and an empty user.
		{"postgres://app:p@ss/w0rd@db.internal:5432/x", []string{"ss/w0rd"}, []string{"postgres://app:", "@db.internal:5432/x"}},
		{"mysql://root:abc/def123@localhost/db", []string{"c/def123"}, []string{"@localhost/db"}},
		{"redis://:s3cretpass@cache:6379", []string{"cretpass"}, []string{"redis://:", "@cache:6379"}},
		// env values: # inside, quotes with spaces and #, lower-case names.
		{"DB_PASSWORD=hunter2#2024", []string{"nter2", "#2024"}, []string{"DB_PASSWORD="}},
		{`SMTP_PASSWORD="correct horse battery staple"`, []string{"horse battery staple"}, []string{`SMTP_PASSWORD="`}},
		{`DB_PASSWORD="abc #def ghi"`, []string{"def ghi"}, []string{"DB_PASSWORD="}},
		{"DB_PASSWORD=p&ssw0rd!", []string{"ssw0rd"}, []string{"DB_PASSWORD="}},
		{"export db_password='lower secret'", []string{"wer secret"}, []string{"db_password="}},
		{"SMTP_PW=pwsecret1\nMYSQL_PWD=pwdsecret2\nSTRIPEKEY=keysecret3\nGOOGLE_CREDENTIALS=credsecret4\nSENTRY_DSN=https://abc123@o1.ingest.example/1",
			[]string{"secret1", "secret2", "secret3", "secret4", "c123@o1"}, []string{"SMTP_PW=", "MYSQL_PWD=", "STRIPEKEY=", "SENTRY_DSN="}},
		// JSON with an escaped quote and a numeric secret.
		{`{"password":"ab\"cdefgh"}`, []string{"cdefgh"}, []string{`{"password":"`}},
		{`{"otp": 123456}`, []string{"3456"}, []string{`"otp"`}},
		// Cookies keep their names.
		{"Cookie: wordpress_logged_in_abc=admin%7C1700000000%7Csecretsessiontoken; theme=darkmode",
			[]string{"secretsessiontoken", "darkmode"}, []string{"wordpress_logged_in_abc=", "; theme="}},
		{"Set-Cookie: laravel_session=eyJpdiI6ImFiY2RlZiJ9; path=/; HttpOnly", []string{"ImFiY2RlZiJ9"}, []string{"laravel_session=", "; path=/; HttpOnly"}},
		{`{"cookie": "sid=abcdef123456"}`, []string{"cdef123456"}, []string{`"sid=`}},
		// JWTs, wherever they appear.
		{"jwt " + jwt, []string{"dozjgNryP4J3jVmNHl0w5N", "eyJzdWIi"}, []string{"jwt "}},
		// Token headers.
		{"x-auth-token: 0123456789abcdef0123", []string{"456789abcdef0123"}, []string{"x-auth-token: "}},
		{"X-Auth-Token: abcdef123456789", []string{"def123456789"}, []string{"X-Auth-Token: "}},
		{"X-WP-Nonce: a1b2c3d4e5", []string{"c3d4e5"}, []string{"X-WP-Nonce: "}},
		{"Authorization: Digest username=\"bob\", response=\"6629fae49393a05397450978507c4ef1\"",
			[]string{`username="bob"`, "6629fae4"}, []string{"Authorization: "}},
		{`{"Authorization": "Bearer abc.def.ghi", "to": "x"}`, []string{"abc.def"}, []string{`"to": "x"`}},
		// Form bodies.
		{"log=admin&pwd=hunter2hunter2&wp-submit=Log+In", []string{"hunter2hunter2"}, []string{"log=admin", "pwd=", "&wp-submit=Log+In"}},
		{"pass1=CorrectHorseBattery&pass2=CorrectHorseBattery", []string{"CorrectHorseBattery"}, []string{"pass1=", "&pass2="}},
		{"Server=db;Database=app;Password=connsecret9;", []string{"nnsecret9"}, []string{"Database=app;", "Password="}},
		// YAML and code.
		{"MAIL_PASSWORD: 'yaml-secret-123'", []string{"secret-123"}, []string{"MAIL_PASSWORD: '"}},
		{"password: yaml-secret-456 # note", []string{"secret-456"}, []string{"password: ", "# note"}},
		{"db:\r\n  password: crlfsecret\r\n  user: app\r\n", []string{"lfsecret"}, []string{"password: ", "\r\n  user: app\r\n"}},
		{"api_key = 'abc123def456'", []string{"c123def456"}, []string{"api_key = '"}},
		{`$config['smtp_pass'] = "phpsecret7";`, []string{"psecret7"}, []string{"smtp_pass"}},
		{`'password' => 'arraysecret8',`, []string{"raysecret8"}, []string{"'password' => '"}},
		// WordPress reset links and reset or magic-link path tokens.
		{"https://shop.example/wp-login.php?action=rp&key=Xy7pQ2rLm9vN3kT8&login=bob", []string{"Xy7pQ2rLm9vN3kT8"}, []string{"action=rp&key=", "&login=bob"}},
		{`<a href="https://shop.example/wp-login.php?login=bob&amp;key=Xy7pQ2rLm9vN3kT8">`, []string{"Xy7pQ2rLm9vN3kT8"}, []string{"login=bob"}},
		{"https://shop.example/wp-login.php?key=Xy7pQ2rLm9vN3kT8&action=resetpass", []string{"Xy7pQ2rLm9vN3kT8"}, []string{"action=resetpass"}},
		{"https://app.example/reset-password/4f9c2b7e1a8d3c6b5e0f9a2d7c4b1e8f?email=a@b.c", []string{"4f9c2b7e1a8d3c6b5e0f9a2d7c4b1e8f"}, []string{"/reset-password/", "?email=a@b.c"}},
		{"https://app.example/email/verify/12/9f8e7d6c5b4a39281706f5e4d3c2b1a0", []string{"9f8e7d6c5b4a3928"}, []string{"/email/verify/12/"}},
		{"https://app.example/magic-link/Zm9vYmFyMTIzNDU2Nzg5MA", []string{"Zm9vYmFyMTIzNDU2Nzg5MA"}, []string{"/magic-link/"}},
		// Private keys, encrypted or not, to the END line.
		{"-----BEGIN RSA PRIVATE KEY-----\nProc-Type: 4,ENCRYPTED\nDEK-Info: AES-128-CBC,0123456789ABCDEF\n\nMIIEowIBAAKCAQEAx1234567890abcdef\n-----END RSA PRIVATE KEY-----\nafter",
			[]string{"ENCRYPTED", "MIIEowIBAAKCAQEAx"}, []string{"-----BEGIN RSA PRIVATE KEY-----", "-----END RSA PRIVATE KEY-----\nafter"}},
		{"-----BEGIN OPENSSH PRIVATE KEY-----\nb3BlbnNzaC1rZXktdjEAAAAA", []string{"b3BlbnNzaC1rZXktdjEAAAAA"}, nil},
		// Plugin options, .npmrc and curl.
		{`a:2:{s:9:"smtp_pass";s:10:"mailpass99";s:9:"smtp_user";s:5:"admin";}`, []string{"ilpass99"}, []string{`s:9:"smtp_pass"`, `s:5:"admin"`}},
		{"//registry.npmjs.org/:_authToken=npmrcsecret5", []string{"rcsecret5"}, []string{":_authToken="}},
		{"curl -u admin:curlsecret6 https://x.example", []string{"rlsecret6"}, []string{"-u admin:"}},
		// Twilio auth tokens next to the account SID or their name.
		{j("account A", "C", hex(32), " token ", cycle("fedcba9876543210", 32)), []string{"fedcba9876543210"}, []string{hex(32)}},
		{"twilio auth token: 0123456789abcdef0123456789abcdef", []string{"0123456789abcdef"}, []string{"twilio auth token: "}},
		// Provider prefixes from the review.
		{j("key sk-", "proj-", mix(40)), []string{mix(40)}, []string{"key "}},
		{j("key sk-", "ant-api03-", mix(40)), []string{mix(40)}, nil},
		{j("gl", "pat-", mix(20)), []string{mix(20)}, nil},
		{j("np", "m_", mix(36)), []string{mix(36)}, nil},
		{j("xa", "pp-1-A0123456789-1234567890123-", hex(64)), []string{hex(64)}, nil},
		{j("sh", "pat_", hex(32)), []string{hex(32)}, nil},
		{j("https://api.telegram.org/bot123456789:A", "A", mix(33), "/sendMessage"), []string{mix(33)}, []string{"/sendMessage"}},
	} {
		got := Redact(c.in)
		for _, g := range c.gone {
			if strings.Contains(got, g) {
				t.Errorf("Redact(%q) = %q still has %q", c.in, got, g)
			}
		}
		for _, k := range c.kept {
			if !strings.Contains(got, k) {
				t.Errorf("Redact(%q) = %q lost %q", c.in, got, k)
			}
		}
	}
}

// TestRedactLeavesNonSecrets holds at least one negative fixture per
// shape: placeholders, prose, cookie attributes and look-alikes.
func TestRedactLeavesNonSecrets(t *testing.T) {
	for _, in := range []string{
		"DB_PASSWORD=\n", "DB_PASSWORD=null\n", "API_KEY=${API_KEY}\n", "APP_NAME=Laravel\n",
		"DB_PASSWORD=changeme\n", "API_KEY=xxxx\n", "SECRET=********\n", "DB_PASSWORD=<your-password>\n",
		"API_TOKEN={{ token }}\n", "DB_PASSWORD=%DB_PASSWORD%\n", "DB_PASSWORD=hu•••••\n", "SECRET_KEY=\n",
		"PASSWORD_MIN_LENGTH=12\n", "TOKEN_TTL=3600\n", "DB_PASSWORD_FILE=/run/secrets/db\n",
		"SORT_KEY=created_at\n", "cacheKey=abcdef\n", "MONKEY=banana\n", "AUTHOR=jane\n", "BYPASS=yes-please\n",
		"password: null\n", "name: my-app\n", "token: none\n", `{"password":""}`, `{"sortKey":"createdAt","author":"jane"}`,
		`{"to":"+15550001111","body":"Your order 1234 has shipped","subject":"Thanks"}`,
		"Click here to finish your password reset.", "Subject: Reset your password",
		"Set-Cookie: a=1; path=/", "Cookie: ", "We use cookies: none",
		"eyJustTesting this", "eyJhbGciOi.notajwt", "eyJhbGciOiJIUzI1NiJ9.eyJzdWIi.short",
		"ssh://git@github.example/repo.git", "https://user@example.com/", "https://example.com:8080/users/@alice",
		"https://example.com/?key=abcdef123456", "https://maps.example/?q=x&key=abcdef123456&login", "text&key=abcdef123456",
		"https://shop.example/confirm/order-summary-page", "https://shop.example/login/", "https://x.example/verify/abc",
		"Authorization: <token>", "X-Request-Id: 12345678",
		"define('WP_DEBUG', true);", `s:5:"title";s:5:"Hello";`, "registry=https://registry.npmjs.org/",
		"curl -u admin https://x.example", "title = 'Hello world'", "page=2&order=desc", "key=value",
		"commit 0123456789abcdef0123456789abcdef", "-----BEGIN PUBLIC KEY-----\nMIIBIjANBgkqhkiG9w0BAQEFAAOCAQ8A\n",
		j("sk-", "proj-short"), j("np", "m_short"), j("gl", "pat-short"),
	} {
		if got := Redact(in); got != in {
			t.Errorf("Redact(%q) = %q, want it unchanged", in, got)
		}
	}
}

func TestStripeTestKeyKeepsKind(t *testing.T) {
	got := FindSecrets([]byte(j("STRIPE_SECRET=sk", "_test_", mix(24))))
	if len(got) != 1 || got[0].Kind != "stripe-key" {
		t.Errorf("got %v, want one stripe-key", got)
	}
}

// ruleFixtures has a positive and a negative fixture for every rule in
// the catalog, keyed by rule ID. TestEveryRuleHasFixtures keeps it
// complete.
var ruleFixtures = map[string]struct{ pos, neg string }{
	"aws-access-key":            {j("id AK", "IA", "ABCDEFGHIJKLMNOP"), j("id AK", "IA", "ABCDEF")},
	"aws-secret-key":            {"aws_secret_access_key = " + mix(40), "aws_secret_access_key = " + mix(12)},
	"google-api-key":            {j("AI", "za", mix(35)), j("AI", "za", mix(10))},
	"google-oauth-secret":       {j("GOC", "SPX-", mix(28)), j("GOC", "SPX-", mix(8))},
	"azure-storage-key":         {"AccountKey=" + mix(86) + "==", "AccountKey=" + mix(20) + "=="},
	"azure-client-secret":       {j("abc8", "Q~", mix(32)), j("abc8", "Q~", mix(8))},
	"digitalocean-token":        {j("do", "p_v1_", hex(64)), j("do", "p_v1_", hex(10))},
	"heroku-api-key":            {j("HR", "KU-", mix(60)), j("HR", "KU-", mix(10))},
	"alibaba-access-key":        {j("LT", "AI", mix(20)), j("LT", "AI", mix(8))},
	"cloudflare-origin-ca-key":  {j("v1.0-", hex(24), "-", hex(146)), j("v1.0-", hex(24), "-", hex(20))},
	"netlify-token":             {j("nf", "p_", mix(36)), j("nf", "p_", mix(10))},
	"vault-token":               {j("hv", "s.", mix(90)), j("hv", "s.", mix(10))},
	"terraform-cloud-token":     {j("abcdefgh123456", ".atlas", "v1.", mix(60)), j("abcdefgh123456", ".atlas", "v1.", mix(10))},
	"doppler-token":             {j("dp", ".pt.", mix(43)), j("dp", ".pt.", mix(10))},
	"pulumi-token":              {j("pu", "l-", hex(40)), j("pu", "l-", hex(10))},
	"databricks-token":          {j("da", "pi", hex(32)), j("da", "pi", hex(10))},
	"supabase-token":            {j("sb", "p_", hex(40)), j("sb", "p_", hex(10))},
	"planetscale-token":         {j("pscale", "_tkn_", mix(43)), j("pscale", "_tkn_", mix(10))},
	"age-secret-key":            {j("AGE-SECRET", "-KEY-1", cycle("QPZRY9X8GF2TVDW0S3JN54KHCE6MUA7L", 58)), j("AGE-SECRET", "-KEY-1", "QPZRY")},
	"onepassword-service-token": {j("op", "s_eyJ", mix(100)), j("op", "s_eyJ", mix(10))},
	"stripe-key":                {j("sk", "_live_", mix(24)), j("sk", "_live_", mix(6))},
	"stripe-webhook-secret":     {j("whs", "ec_", mix(24)), j("whs", "ec_", mix(6))},
	"square-token":              {j("EA", "AA", mix(60)), j("EA", "AA", mix(10))},
	"braintree-token":           {j("access_token$", "production$", "abcdefgh12345678$", hex(32)), "access_token$production$short"},
	"shopify-token":             {j("sh", "pat_", hex(32)), j("sh", "pat_", hex(10))},
	"flutterwave-key":           {j("FLWSECK", "_TEST-", cycle("abcdefgh01234567", 32), "-X"), j("FLWSECK", "_TEST-", "abc-X")},
	"easypost-key":              {j("EZ", "AK", mix(54)), j("EZ", "AK", mix(10))},
	"shippo-token":              {j("shippo", "_live_", hex(40)), j("shippo", "_live_", hex(10))},
	"duffel-token":              {j("duffel", "_test_", mix(43)), j("duffel", "_test_", mix(10))},
	"slack-token":               {j("xo", "xb-", "1234567890-abcdefghij"), j("xo", "xb-", "123")},
	"slack-app-token":           {j("xa", "pp-1-A0123456789-1234567890123-", hex(64)), j("xa", "pp-x")},
	"slack-webhook":             {j("https://hooks.slack.com/services/T0001/B0002/", "XXXXabcd", "YYYYefgh"), "https://hooks.slack.com/services/T0001/"},
	"discord-webhook":           {j("https://discord.com/api/webhooks/123456789012345678/", mix(68)), "https://discord.com/api/webhooks/123/abc"},
	"discord-bot-token":         {j("M", mix(23), ".", mix(6), ".", mix(27)), j("M", mix(23), ".", mix(6), ".", mix(5))},
	"teams-webhook":             {j("https://contoso.webhook.office.com/webhookb2/", mix(40)), "https://contoso.webhook.office.com/webhookb2/short"},
	"telegram-bot-token":        {j("https://api.telegram.org/bot123456789:A", "A", mix(33), "/getMe"), j("123456789:A", "Ashort")},
	"twilio-api-key":            {j("S", "K", hex(32)), j("S", "K", hex(10))},
	"twilio-auth-token":         {j("A", "C", hex(32), " ", cycle("fedcba9876543210", 32)), "auth " + cycle("fedcba9876543210", 32)},
	"sendgrid-key":              {j("S", "G.", mix(22), ".", mix(43)), j("S", "G.", mix(5))},
	"mailgun-key":               {j("ke", "y-", hex(32)), j("ke", "y-", hex(10))},
	"mailchimp-key":             {j(hex(32), "-us6"), j(hex(10), "-us6")},
	"brevo-key":                 {j("xkey", "sib-", hex(64), "-", mix(16)), j("xkey", "sib-", hex(10))},
	"resend-key":                {j("r", "e_", mix(24)), j("r", "e_", mix(6))},
	"github-token":              {j("gh", "p_", mix(36)), j("gh", "p_", mix(10))},
	"github-fine-grained-token": {j("github", "_pat_", mix(82)), j("github", "_pat_", mix(10))},
	"gitlab-token":              {j("gl", "pat-", mix(20)), j("gl", "pat-", mix(6))},
	"atlassian-token":           {j("ATA", "TT3", mix(120)), j("ATA", "TT3", mix(10))},
	"npm-token":                 {j("np", "m_", mix(36)), j("np", "m_", mix(10))},
	"pypi-token":                {j("py", "pi-AgEIcHlwaS5vcmc", mix(60)), j("py", "pi-AgEIcHlwaS5vcmc", mix(10))},
	"rubygems-key":              {j("rubygems", "_", hex(48)), j("rubygems", "_", hex(10))},
	"dockerhub-token":           {j("dckr", "_pat_", mix(27)), j("dckr", "_pat_", mix(10))},
	"nuget-key":                 {j("oy", "2", hex(43)), j("oy", "2", hex(10))},
	"circleci-token":            {j("CCI", "PAT_", mix(22), "_", hex(40)), j("CCI", "PAT_", mix(6))},
	"buildkite-token":           {j("bk", "ua_", hex(40)), j("bk", "ua_", hex(10))},
	"sonarqube-token":           {j("sq", "u_", hex(40)), j("sq", "u_", hex(10))},
	"jfrog-key":                 {j("AK", "Cp", mix(69)), j("AK", "Cp", mix(10))},
	"grafana-token":             {j("gl", "c_", mix(40)), j("gl", "c_", mix(10))},
	"sentry-token":              {j("sntry", "u_", hex(64)), j("sntry", "u_", hex(10))},
	"newrelic-key":              {j("NR", "AK-", "ABCDEFGHIJ0123456789KLMNOPQ"), j("NR", "AK-", "ABC")},
	"linear-key":                {j("lin", "_api_", mix(40)), j("lin", "_api_", mix(10))},
	"notion-token":              {j("nt", "n_", digits(11), mix(35)), j("nt", "n_", digits(11))},
	"figma-token":               {j("fi", "gd_", mix(40)), j("fi", "gd_", mix(10))},
	"airtable-token":            {j("pa", "t", mix(14), ".", hex(64)), j("pa", "t", mix(14), ".", hex(10))},
	"hubspot-token":             {j("pat", "-na1-", "0123abcd-0123-4567-89ab-0123456789ab"), j("pat", "-na1-", "0123")},
	"postman-key":               {j("PM", "AK-", hex(24), "-", hex(34)), j("PM", "AK-", hex(10))},
	"contentful-token":          {j("CF", "PAT-", mix(43)), j("CF", "PAT-", mix(10))},
	"typeform-token":            {j("tf", "p_", mix(44)), j("tf", "p_", mix(10))},
	"dropbox-token":             {j("s", "l.", mix(120)), j("s", "l.", mix(10))},
	"mapbox-token":              {j("s", "k.eyJ", mix(60), ".", mix(22)), j("s", "k.eyJ", mix(10))},
	"anthropic-key":             {j("sk-", "ant-api03-", mix(40)), j("sk-", "ant-", mix(5))},
	"openrouter-key":            {j("sk-", "or-v1-", hex(64)), j("sk-", "or-v1-", hex(10))},
	"openai-key":                {j("sk-", "proj-", mix(40)), j("sk-", "proj-", mix(5))},
	"huggingface-token":         {j("h", "f_", letters(34)), j("h", "f_", letters(10))},
	"replicate-token":           {j("r", "8_", mix(37)), j("r", "8_", mix(10))},
	"groq-key":                  {j("gs", "k_", mix(52)), j("gs", "k_", mix(10))},
	"perplexity-key":            {j("pp", "lx-", mix(48)), j("pp", "lx-", mix(10))},
	"xai-key":                   {j("xa", "i-", mix(80)), j("xa", "i-", mix(10))},
	"private-key":               {"-----BEGIN EC PRIVATE KEY-----\nMHcCAQEEIBkg4LVWM9nuwNSk\n-----END EC PRIVATE KEY-----", "-----BEGIN PUBLIC KEY-----\nMHcCAQEEIBkg4LVWM9nuwNSk\n"},
	"jwt":                       {"eyJhbGciOiJIUzI1NiJ9.eyJzdWIiOiIxMjM0In0.dozjgNryP4J3jVmNHl0w5N", "eyJhbGciOi.notajwt"},
	"url-credentials":           {"https://bob:hunter22@example.com/", "https://bob@example.com/"},
	"authorization":             {"Authorization: Bearer abcdef123456", "Authorization: <token>"},
	"cookie":                    {"Cookie: sid=abcdef123456", "Cookie: path=/"},
	"header-token":              {"X-Api-Key: abcdef123456", "X-Request-Id: abcdef123456"},
	"wp-reset-key":              {"/wp-login.php?action=rp&key=Xy7pQ2rLm9vN3kT8&login=bob", "/?key=Xy7pQ2rLm9vN3kT8"},
	"reset-link-token":          {"/reset-password/4f9c2b7e1a8d3c6b5e0f9a2d", "/reset-password/order-summary-page"},
	"wp-config":                 {"define('AUTH_KEY', 'put-something-unique-1');", "define('WP_DEBUG', 'yes-please');"},
	"php-serialized":            {`s:9:"smtp_pass";s:10:"mailpass99";`, `s:5:"title";s:5:"Hello";`},
	"npmrc-auth":                {"//registry.npmjs.org/:_authToken=npmrcsecret5", "//registry.npmjs.org/:always-auth=true"},
	"curl-user":                 {"curl -u admin:curlsecret6 https://x", "curl -u admin https://x"},
	"env":                       {"DB_PASSWORD=supersecret", "APP_NAME=Laravel"},
	"yaml":                      {"password: yaml-secret-456", "name: my-app"},
	"json-field":                {`{"clientSecret":"abc123def456"}`, `{"sortKey":"createdAt"}`},
	"assignment":                {`$db_password = "abc123def456";`, `title = 'Hello world'`},
	"form-field":                {"a=1&pwd=hunter2hunter2", "page=2&order=desc"},
}

func TestEveryRuleHasFixtures(t *testing.T) {
	rules := Rules()
	if len(rules) < 80 {
		t.Errorf("catalog has %d rules, want about 80 or more", len(rules))
	}
	seen := map[string]bool{}
	for _, r := range rules {
		if seen[r.ID] {
			t.Errorf("duplicate rule ID %s", r.ID)
		}
		seen[r.ID] = true
		if r.Description == "" || len(r.Tags) == 0 {
			t.Errorf("%s: needs a description and tags", r.ID)
		}
		f, ok := ruleFixtures[r.ID]
		if !ok {
			t.Errorf("%s: no fixtures", r.ID)
			continue
		}
		kinds := func(s string) []string {
			var out []string
			for _, sec := range FindSecrets([]byte(s)) {
				out = append(out, sec.Kind)
			}
			return out
		}
		if got := kinds(f.pos); !slices.Contains(got, r.ID) {
			t.Errorf("%s: positive fixture %q found %v", r.ID, f.pos, got)
		}
		if got := kinds(f.neg); slices.Contains(got, r.ID) {
			t.Errorf("%s: negative fixture %q was matched", r.ID, f.neg)
		}
	}
	for id := range ruleFixtures {
		if !seen[id] {
			t.Errorf("fixtures for unknown rule %s", id)
		}
	}
	// Callers can't change the catalog through the listing.
	rules[0].Tags[0] = "changed"
	if Rules()[0].Tags[0] == "changed" {
		t.Error("Rules must return copies")
	}
}

func TestSecretString(t *testing.T) {
	if got := (Secret{Kind: "env", Name: "DB_PASSWORD", Masked: "su•••••"}).String(); got != "DB_PASSWORD=su•••••" {
		t.Errorf("String = %q", got)
	}
	if got := (Secret{Kind: "jwt", Masked: "eyJ•••••"}).String(); got != "eyJ•••••" {
		t.Errorf("String = %q", got)
	}
}

func TestFindSecretsDedupes(t *testing.T) {
	got := FindSecrets([]byte("DB_PASSWORD=supersecret\nDB_PASSWORD=supersecret\n"))
	if len(got) != 1 {
		t.Errorf("got %v, want one", got)
	}
}

func TestMask(t *testing.T) {
	for _, c := range []struct {
		v    string
		keep int
		want string
	}{
		{"supersecret", 2, "su•••••"},
		{"abc", 8, "a•••••"},
		{"sk_live_abcdefghijklmnop", 8, "sk_l•••••"}, // never more than 4
		{"secret", -1, "•••••"},
	} {
		if got := Mask(c.v, c.keep); got != c.want {
			t.Errorf("Mask(%q, %d) = %q, want %q", c.v, c.keep, got, c.want)
		}
	}
}

func TestIsSecretName(t *testing.T) {
	for name, want := range map[string]bool{
		"DB_PASSWORD": true, "clientSecret": true, "dbUserPwd": true, "pass2": true, "SMTP_PW": true,
		"STRIPEKEY": true, "license_key": true, "SENTRY_DSN": true, "user[password]": true, "auth": true,
		"key": false, "sort_key": false, "cacheKey": false, "MONKEY": false, "BYPASS": false,
		"PASSWORD_MIN_LENGTH": false, "AUTHOR": false, "PWA_ENABLED": false, "": false, "__": false,
	} {
		if got := isSecretName(name); got != want {
			t.Errorf("isSecretName(%q) = %v, want %v", name, got, want)
		}
	}
}

func TestRedactHeader(t *testing.T) {
	for _, c := range []struct{ name, value, want string }{
		{"Authorization", "Digest username=\"bob\"", "•••••"},
		{"X-WP-Nonce", "a1b2c3d4e5", "•••••"},
		{"x-auth-token", "abc", "•••••"},
		{"Api-Key", "abcdef", "•••••"},
		{"Client-Secret", "abcdef", "•••••"},
		{"Cookie", "sid=abcdef123; theme=dark; x=", "sid=•••••; theme=•••••; x="},
		{"Set-Cookie", "laravel_session=eyJpdiI6; Path=/; HttpOnly", "laravel_session=•••••; Path=/; HttpOnly"},
		{"Content-Type", "text/html", "text/html"},
		{"Referer", "https://bob:hunter22@example.com/", "https://bob:hu•••••@example.com/"},
		{"Authorization", "  ", "  "},
	} {
		if got := RedactHeader(c.name, c.value); got != c.want {
			t.Errorf("RedactHeader(%q, %q) = %q, want %q", c.name, c.value, got, c.want)
		}
	}
}

func BenchmarkRedact(b *testing.B) {
	page := strings.Repeat(`<p>Hello, your order <a href="https://shop.example/order/123?ref=mail">123</a> has shipped.</p>
{"to":"+15550001111","subject":"Thanks","password":"abc123def456"}
`, 2000)
	b.SetBytes(int64(len(page)))
	for b.Loop() {
		Redact(page)
	}
}
