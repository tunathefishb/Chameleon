package tui

import (
	"math/rand"
	"net"
	"strings"
	"sync"
	"testing"
	"unicode/utf8"

	"github.com/charmbracelet/lipgloss"
)

// TestChallengerStressTruncateRunes_ExhaustiveWidth runs an exhaustive matrix of
// complex, hostile, and multi-byte Unicode strings across maxLen -2..35.
// Invariants verified:
// 1. Zero slice bounds panics.
// 2. lipgloss.Width(res) <= maxLen (for maxLen >= 0).
// 3. res == "" (for maxLen <= 0).
// 4. utf8.ValidString(res) is true.
// 5. Zero \ufffd replacement runes introduced.
// 6. When ellipsis "..." is appended, res does not end in dangling ZWJ (\u200D) before ellipsis.
// 7. When ellipsis "..." is appended, res does not end in dangling variation selectors (\uFE00..\uFE0F) before ellipsis.
func TestChallengerStressTruncateRunes_ExhaustiveWidth(t *testing.T) {
	testCases := []struct {
		category string
		input    string
	}{
		// 1. Standard ASCII
		{"ascii_empty", ""},
		{"ascii_single", "a"},
		{"ascii_short", "hello"},
		{"ascii_sentence", "The quick brown fox jumps over the lazy dog."},
		{"ascii_long", "A very long ASCII string that exceeds typical column bounds by a large margin"},

		// 2. Whitespace & Control Characters
		{"ws_single_space", " "},
		{"ws_multi_space", "     "},
		{"ws_mixed", " \t \r\n "},
		{"ws_leading_trailing", "   centered text   "},
		{"ctrl_null", "null\x00byte"},
		{"ctrl_bell_bs", "bell\x07backspace\x08text"},

		// 3. Accented & 2-Byte UTF-8
		{"accent_cafe", "Café au lait & résumé"},
		{"accent_german", "Übergrößenträger müssen vorsichtig sein"},
		{"accent_spanish", "¿Cómo estás, señor? ¡Muy bien!"},
		{"script_cyrillic", "Привет, как поживает Chameleon?"},
		{"script_greek", "Καλημέρα κόσμε! Ελληνικό κείμενο"},
		{"script_arabic", "مرحبا بكم في اختبار الزواحف والترميز"},
		{"script_hebrew", "שלום לכולם, בדיקת מחרוזת בעברית"},

		// 4. CJK & Full-Width Characters
		{"cjk_chinese_simp", "你好世界，欢迎使用 Chameleon 爬虫引擎！"},
		{"cjk_chinese_trad", "歡迎使用變色龍爬蟲工具，繁體中文測試"},
		{"cjk_japanese_hira", "こんにちは世界、日本語の文字列テストです"},
		{"cjk_japanese_kata", "コンニチハ、クローラーノテストデス"},
		{"cjk_korean_hangul", "안녕하세요 세계, 고성능 크롤러 테스트"},
		{"cjk_fullwidth_punct", "！？【】（）《》「」『』、。：；"},
		{"cjk_fullwidth_ascii", "ＡＢＣＤＥＦＧＨＩＪＫＬＭＮＯＰＱＲＳＴＵＶＷＸＹＺ"},
		{"cjk_halfwidth_kata", "ｱｲｳｴｵｶｷｸｹｺｻｼｽｾｿ"},

		// 5. Standard 4-Byte Emojis
		{"emoji_single_fire", "🔥"},
		{"emoji_single_rocket", "🚀"},
		{"emoji_chain", "🔥🚀🍕🦀🌈💎🦄🎉🐱🐶🦊🐼🐨🐯🦁"},
		{"emoji_mixed_text", "Status: 🚀 Launching... 🍕 Done!"},

		// 6. Emoji Variation Selectors (VS-15 Text, VS-16 Emoji)
		{"vs16_warning", "⚠️"},
		{"vs16_warning_text", "⚠️ Alert: Critical system warning!"},
		{"vs16_heart", "❤️"},
		{"vs16_heart_text", "❤️ Love and peace to all"},
		{"vs16_telephone", "☎️ Support: 1-800-CHAMELEON"},
		{"vs16_envelope", "✉️ Contact: admin@chameleon.local"},
		{"vs16_airplane", "✈️ Flight: departures and arrivals"},
		{"vs16_weather", "☀️ Sunny with ⛅ partly cloudy skies 🌧️"},
		{"vs15_text_warning", "⚠️\uFE0E"},
		{"vs15_text_heart", "❤️\uFE0E"},

		// 7. Emoji Skin Tone Modifiers (U+1F3FB .. U+1F3FF)
		{"skin_thumbs_light", "👍🏻"},
		{"skin_thumbs_medlight", "👍🏼"},
		{"skin_thumbs_med", "👍🏽"},
		{"skin_thumbs_meddark", "👍🏾"},
		{"skin_thumbs_dark", "👍🏿"},
		{"skin_waving_dark", "👋🏿 Hello there!"},
		{"skin_person_med", "🧑🏽 Working remotely"},

		// 8. Keycap Sequences (Number + VS-16 + U+20E3)
		{"keycap_zero", "0️⃣ Zero"},
		{"keycap_one", "1️⃣ First step"},
		{"keycap_two", "2️⃣ Second step"},
		{"keycap_hash", "#️⃣ Channel header"},
		{"keycap_star", "*️⃣ Asterisk point"},
		{"keycap_multi", "1️⃣ 2️⃣ 3️⃣ 4️⃣ 5️⃣ 6️⃣ 7️⃣ 8️⃣ 9️⃣ 🔟"},

		// 9. Regional Indicator Flags (ISO 3166-1)
		{"flag_us", "🇺🇸 United States"},
		{"flag_uk", "🇬🇧 Great Britain"},
		{"flag_jp", "🇯🇵 Japan"},
		{"flag_de", "🇩🇪 Germany"},
		{"flag_fr", "🇫🇷 France"},
		{"flag_br", "🇧🇷 Brazil"},
		{"flag_multi", "🇺🇸 🇬🇧 🇯🇵 🇩🇪 🇫🇷 🇧🇷 🇮🇳 🇨🇳 🇦🇺 🇨🇦"},

		// 10. ZWJ Sequences (Complex Graphemes)
		{"zwj_family_4", "👨‍👩‍👧‍👦"},
		{"zwj_family_3", "👨‍👩‍👦"},
		{"zwj_family_women", "👩‍👩‍👧"},
		{"zwj_pride_flag", "🏳️‍🌈"},
		{"zwj_trans_flag", "🏳️‍⚧️"},
		{"zwj_coder_woman", "👩‍💻"},
		{"zwj_coder_man", "👨‍💻"},
		{"zwj_scientist", "👩‍🔬"},
		{"zwj_astronaut", "👨‍🚀"},
		{"zwj_holding_hands", "🧑‍🤝‍🧑"},
		{"zwj_polar_bear", "🐻‍❄️"},
		{"zwj_black_cat", "🐈‍⬛"},
		{"zwj_eye_speech", "👁️‍🗨️"},
		{"zwj_farmer_skin", "👨🏽‍🌾 Farmer at work"},
		{"zwj_mixed_sentence", "Team: 👩‍💻 Dev, 👨‍🔬 Lab, 👨‍🚀 Ops under 🏳️‍🌈 banner!"},

		// 11. Combining Diacritics & Zalgo
		{"zalgo_heavy", "Z\u0301\u0302\u0303a\u0300\u0308l\u0304g\u0305o\u0306 text test"},
		{"combining_accents_only", "\u0300\u0301\u0302\u0303\u0304"},
		{"combining_e_acute", "e\u0301 vs é equivalence"},

		// 12. ANSI Escape Sequences
		{"ansi_16_red", "\x1b[31mRed text\x1b[0m"},
		{"ansi_16_bold_green", "\x1b[1;32mBold green\x1b[0m"},
		{"ansi_16_underline_blue", "\x1b[4;34mUnderlined blue\x1b[0m"},
		{"ansi_256_color", "\x1b[38;5;208mOrange 256 color\x1b[0m"},
		{"ansi_truecolor_rgb", "\x1b[38;2;120;220;50mCustom RGB green\x1b[0m"},
		{"ansi_compound", "\x1b[1;31;43mBold red on yellow\x1b[0m normal"},
		{"ansi_unterminated", "\x1b[31mUnclosed color sequence"},
		{"ansi_bare_fragments", "\x1b[\x1b]33;text\x1b[m"},
		{"ansi_with_emoji", "\x1b[32m🚀 Success\x1b[0m \x1b[31m⚠️ Error\x1b[0m"},
		{"ansi_with_cjk", "\x1b[36m中文測試\x1b[0m \x1b[35m日本語\x1b[0m"},

		// 13. Mixed Hostile Permutations
		{"mixed_all", "⚠️ [WARN] 🚀 爬虫启动 \x1b[32mOK\x1b[0m 👨‍👩‍👧‍👦 (1️⃣ of 🔟) Z\u0301algo"},
		{"mixed_url", "https://example.com/über/café?q=🔥🚀&cat=🦀&tag=日本語#section1"},
	}

	for _, tc := range testCases {
		tc := tc
		t.Run(tc.category, func(t *testing.T) {
			for maxLen := -2; maxLen <= 35; maxLen++ {
				var res string
				func() {
					defer func() {
						if r := recover(); r != nil {
							t.Fatalf("PANIC on input %q (%s) with maxLen %d: %v", tc.input, tc.category, maxLen, r)
						}
					}()
					res = truncateRunes(tc.input, maxLen)
				}()

				// Invariant 1: UTF-8 validity
				if !utf8.ValidString(res) {
					t.Fatalf("truncateRunes(%q, %d) produced INVALID UTF-8 bytes: %x", tc.input, maxLen, res)
				}

				// Invariant 2: No \ufffd introduced
				if strings.ContainsRune(res, '\ufffd') && !strings.ContainsRune(tc.input, '\ufffd') {
					t.Fatalf("truncateRunes(%q, %d) introduced \\ufffd replacement character: %q", tc.input, maxLen, res)
				}

				// Invariant 3: Negative or zero maxLen yields empty string
				if maxLen <= 0 {
					if res != "" {
						t.Fatalf("truncateRunes(%q, %d) = %q, want empty string", tc.input, maxLen, res)
					}
					continue
				}

				// Invariant 4: Strict adherence to visual width limit
				w := lipgloss.Width(res)
				if w > maxLen {
					t.Fatalf("VIOLATION [VISUAL WIDTH OVERFLOW]: truncateRunes(%q, maxLen=%d) = %q has visual width %d > %d",
						tc.input, maxLen, res, w, maxLen)
				}

				// Invariant 5: When ellipsis "..." is appended, no dangling ZWJ (\u200D) before ellipsis
				if strings.HasSuffix(res, "...") {
					prefix := strings.TrimSuffix(res, "...")
					if strings.HasSuffix(prefix, "\u200D") {
						t.Fatalf("VIOLATION [DANGLING ZWJ]: truncateRunes(%q, %d) = %q leaves dangling \\u200D before ellipsis",
							tc.input, maxLen, res)
					}

					// Invariant 6: When ellipsis "..." is appended, no dangling variation selector (\uFE00..\uFE0F) before ellipsis
					if len(prefix) > 0 {
						lastRune, _ := utf8.DecodeLastRuneInString(prefix)
						if lastRune >= '\uFE00' && lastRune <= '\uFE0F' {
							t.Fatalf("VIOLATION [DANGLING VARIATION SELECTOR]: truncateRunes(%q, %d) = %q leaves dangling \\u%04X before ellipsis",
								tc.input, maxLen, res, lastRune)
						}
					}
				}
			}
		})
	}
}

// TestChallengerStressURLValidation_AdversarialSuite tests normalizeAndValidateURL
// across an exhaustive matrix of hostile schemes, IPv6, userinfo, and malformed hosts.
func TestChallengerStressURLValidation_AdversarialSuite(t *testing.T) {
	type urlCase struct {
		name        string
		input       string
		expectError bool
		checkHost   string
		desc        string
	}

	cases := []urlCase{
		// --- 1. Hostile Schemes ---
		{"scheme_javascript_simple", "javascript:alert(1)", true, "", "standard XSS javascript scheme"},
		{"scheme_javascript_caps", "JAVASCRIPT:alert(1)", true, "", "uppercase javascript scheme"},
		{"scheme_javascript_void", "javascript:void(0)", true, "", "javascript void expression"},
		{"scheme_data_html", "data:text/html,<script>alert(1)</script>", true, "", "data URI with HTML payload"},
		{"scheme_data_base64", "data:text/plain;base64,SGVsbG8=", true, "", "data URI base64"},
		{"scheme_data_caps", "DATA:text/html,test", true, "", "uppercase data URI"},
		{"scheme_file_passwd", "file:///etc/passwd", true, "", "local file scheme unix"},
		{"scheme_file_win", "file:///C:/Windows/win.ini", true, "", "local file scheme windows"},
		{"scheme_file_localhost", "file://localhost/etc/shadow", true, "", "file scheme with localhost"},
		{"scheme_vbscript", "vbscript:msgbox(1)", true, "", "vbscript pseudo-protocol"},
		{"scheme_mailto_basic", "mailto:user@domain.com", true, "", "mailto scheme"},
		{"scheme_mailto_query", "mailto:user@domain.com?subject=hello", true, "", "mailto with query params"},
		{"scheme_tel", "tel:+1234567890", true, "", "telephone URI"},
		{"scheme_ftp", "ftp://ftp.is.co.za/rfc/rfc1808.txt", true, "", "ftp scheme"},
		{"scheme_ftp_userpass", "ftp://user:pass@ftp.example.com", true, "", "ftp with credentials"},
		{"scheme_ws", "ws://echo.websocket.events", true, "", "websocket ws scheme"},
		{"scheme_wss", "wss://echo.websocket.events", true, "", "secure websocket wss scheme"},
		{"scheme_ssh", "ssh://git@github.com", true, "", "ssh scheme"},
		{"scheme_git", "git://github.com/project/repo.git", true, "", "git scheme"},
		{"scheme_gopher", "gopher://gopher.floodgap.com", true, "", "gopher scheme"},
		{"scheme_ldap", "ldap://ldap.example.com", true, "", "ldap scheme"},
		{"scheme_about", "about:blank", true, "", "about blank scheme"},
		{"scheme_blob", "blob:https://example.com/uuid", true, "", "blob URI"},
		{"scheme_irc", "irc://irc.libera.chat:6667", true, "", "irc scheme"},
		{"scheme_news", "news:comp.lang.go", true, "", "news scheme"},
		{"scheme_custom_double_slash", "custom://api.service.com", true, "", "custom scheme with ://"},
		{"scheme_php_wrapper", "php://filter/resource=index.php", true, "", "php stream wrapper"},

		// --- 2. Bracketed IPv6 Addresses ---
		{"ipv6_loopback_bare", "[::1]", false, "[::1]", "bare bracketed IPv6 loopback"},
		{"ipv6_loopback_http", "http://[::1]", false, "[::1]", "http bracketed IPv6 loopback"},
		{"ipv6_loopback_https", "https://[::1]", false, "[::1]", "https bracketed IPv6 loopback"},
		{"ipv6_loopback_port", "http://[::1]:8080", false, "[::1]:8080", "bracketed IPv6 with port"},
		{"ipv6_loopback_path", "https://[::1]:8443/api/v1/health", false, "[::1]:8443", "bracketed IPv6 with path"},
		{"ipv6_global_unicast", "http://[2001:db8:85a3::8a2e:370:7334]", false, "[2001:db8:85a3::8a2e:370:7334]", "bracketed global unicast IPv6"},
		{"ipv6_global_with_port", "https://[2001:db8:85a3::8a2e:370:7334]:443", false, "[2001:db8:85a3::8a2e:370:7334]:443", "bracketed global IPv6 with port"},
		{"ipv6_ipv4_mapped", "http://[::ffff:192.0.2.128]:80", false, "[::ffff:192.0.2.128]:80", "bracketed IPv4-mapped IPv6"},
		{"ipv6_all_zeroes", "http://[::]:8080", false, "[::]:8080", "bracketed all-zeroes IPv6 with port"},
		{"ipv6_link_local", "http://[fe80::1]", false, "[fe80::1]", "bracketed link-local IPv6"},

		// --- 3. Malformed & Unbracketed IPv6 (RFC 3986 requires brackets) ---
		{"ipv6_unbracketed_loopback_http", "http://::1", true, "", "unbracketed IPv6 in HTTP URL (too many colons)"},
		{"ipv6_unbracketed_loopback_bare", "::1", true, "", "bare unbracketed IPv6"},
		{"ipv6_unbracketed_full", "http://2001:db8::1", true, "", "unbracketed full IPv6"},
		{"ipv6_unclosed_bracket", "http://[::1", true, "", "unclosed bracket in IPv6"},
		{"ipv6_missing_opening_bracket", "http://::1]", true, "", "missing opening bracket in IPv6"},
		{"ipv6_double_bracketed", "http://[[::1]]", true, "", "double bracketed IPv6"},
		{"ipv6_invalid_content", "http://[not-an-ip]", true, "", "non-IP content inside brackets"},
		{"ipv6_invalid_hex", "http://[gggg::1]", true, "", "invalid hex digits in bracketed IPv6"},

		// --- 4. Userinfo & Embedded Credentials ---
		{"userinfo_user_pass", "https://user:password@example.com", false, "example.com", "standard user:pass credentials"},
		{"userinfo_user_only", "http://crawler@example.org/feed", false, "example.org", "user only without password"},
		{"userinfo_percent_encoded_pass", "https://admin:p%40ssw0rd%21%23%24@example.com/admin", false, "example.com", "percent-encoded password characters"},
		{"userinfo_unencoded_hash_rejected", "https://admin:p@ssw0rd!#$@example.com/admin", true, "", "unencoded hash splits URL into fragment per RFC 3986"},
		{"userinfo_with_port", "http://alice:secret@127.0.0.1:8080/dashboard", false, "127.0.0.1:8080", "credentials with IPv4 and port"},
		{"userinfo_with_ipv6", "http://user:pass@[::1]:9000/metrics", false, "[::1]:9000", "credentials with IPv6 and port"},
		{"userinfo_empty_pass", "http://user:@example.com", false, "example.com", "empty password userinfo"},
		{"userinfo_empty_user", "http://:password@example.com", false, "example.com", "empty username userinfo"},

		// --- 5. Malformed Hosts & Authority ---
		{"malformed_empty", "", true, "", "empty string"},
		{"malformed_spaces_only", "   ", true, "", "spaces only"},
		{"malformed_tabs_newlines", "\t\r\n", true, "", "whitespace control characters"},
		{"malformed_bare_http", "http://", true, "", "bare http scheme"},
		{"malformed_bare_https", "https://", true, "", "bare https scheme"},
		{"malformed_triple_slash", "http:///", true, "", "http with three slashes"},
		{"malformed_quintuple_slash", "https://///", true, "", "https with five slashes"},
		{"malformed_spaces_in_host", "http://example .com", true, "", "space in host name"},
		{"malformed_header_injection_crlf", "http://example.com\r\nHost: evil.com", true, "", "CRLF header injection in host"},
		{"malformed_newline_in_middle_host", "http://foo\nbar.com", true, "", "newline inside host label"},
		{"malformed_tab_in_host", "http://example\t.com", true, "", "tab in host"},
		{"malformed_backslash_host", "http://example.com\\test", true, "", "backslash in host/authority"},
		{"malformed_non_numeric_port", "http://example.com:abc", true, "", "non-numeric port letters"},
		{"malformed_negative_port", "http://example.com:-80", true, "", "negative port number"},
		{"malformed_single_label_no_dot", "internalserver", true, "", "single label without dot (not localhost)"},
		{"malformed_single_label_http", "http://myintranet", true, "", "http single label without dot"},

		// --- 6. Valid Targets (IPv4, Localhost, FQDNs, Ports) ---
		{"valid_domain_bare", "example.com", false, "example.com", "bare domain name"},
		{"valid_domain_subdomain", "sub.api.example.com", false, "sub.api.example.com", "subdomain bare"},
		{"valid_domain_http", "http://example.com", false, "example.com", "explicit http domain"},
		{"valid_domain_https", "https://example.com", false, "example.com", "explicit https domain"},
		{"valid_domain_port", "https://example.com:8443", false, "example.com:8443", "explicit https with custom port"},
		{"valid_domain_path_query_frag", "https://example.com:443/search?q=test#frag", false, "example.com:443", "full URL with path query fragment"},
		{"valid_localhost_bare", "localhost", false, "localhost", "bare localhost"},
		{"valid_localhost_http", "http://localhost", false, "localhost", "http localhost"},
		{"valid_localhost_port", "localhost:3000", false, "localhost:3000", "bare localhost with port"},
		{"valid_localhost_https_port", "https://localhost:8080", false, "localhost:8080", "https localhost with port"},
		{"valid_ipv4_bare", "127.0.0.1", false, "127.0.0.1", "bare IPv4 loopback"},
		{"valid_ipv4_http", "http://127.0.0.1", false, "127.0.0.1", "http IPv4 loopback"},
		{"valid_ipv4_port", "192.168.1.1:8080", false, "192.168.1.1:8080", "bare private IPv4 with port"},
		{"valid_ipv4_path", "http://10.0.0.1:9000/status", false, "10.0.0.1:9000", "private IPv4 with path"},
		{"valid_trailing_newline_trimmed", "http://example.com\n", false, "example.com", "trailing newline cleanly trimmed by TrimSpace"},
		{"valid_port_trailing_colon_omitted", "http://example.com:", false, "example.com:", "omitted port with trailing colon accepted by standard Go net/url"},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			norm, parsed, err := normalizeAndValidateURL(tc.input)

			if tc.expectError {
				if err == nil {
					t.Fatalf("VIOLATION: normalizeAndValidateURL(%q) expected error for %s, but succeeded with norm=%q host=%q",
						tc.input, tc.desc, norm, parsed.Host)
				}
			} else {
				if err != nil {
					t.Fatalf("VIOLATION: normalizeAndValidateURL(%q) unexpected error for %s: %v",
						tc.input, tc.desc, err)
				}
				if parsed == nil {
					t.Fatalf("VIOLATION: normalizeAndValidateURL(%q) returned nil *url.URL", tc.input)
				}
				if tc.checkHost != "" && parsed.Host != tc.checkHost {
					t.Fatalf("VIOLATION: normalizeAndValidateURL(%q) parsed.Host = %q, want %q",
						tc.input, parsed.Host, tc.checkHost)
				}
				if !strings.HasPrefix(norm, "http://") && !strings.HasPrefix(norm, "https://") {
					t.Fatalf("VIOLATION: normalizeAndValidateURL(%q) norm %q missing http/https scheme",
						tc.input, norm)
				}
				if strings.ContainsAny(parsed.Host, " \t\r\n") {
					t.Fatalf("VIOLATION: normalizeAndValidateURL(%q) parsed.Host %q contains whitespace",
						tc.input, parsed.Host)
				}
			}
		})
	}
}

// TestChallengerStressURLPropertyFuzz feeds 2,000 pseudo-random hostile permutations
// into normalizeAndValidateURL to guarantee zero panics and invariant preservation.
func TestChallengerStressURLPropertyFuzz(t *testing.T) {
	rng := rand.New(rand.NewSource(133742))

	schemes := []string{
		"", "http://", "https://", "ftp://", "file://", "javascript:", "data:", "vbscript:",
		"mailto:", "tel:", "ws://", "wss://", "custom://", "php://", "//",
	}

	users := []string{
		"", "admin@", "user:pass@", "foo:bar:baz@", "@", ":pass@", "user:@",
	}

	hosts := []string{
		"", "example.com", "sub.domain.co.uk", "localhost", "127.0.0.1", "192.168.1.10",
		"[::1]", "[2001:db8::1]", "[::ffff:192.0.2.1]", "::1", "[notanip]", "singlelabel",
		"host with space.com", "host\nname.com", "host\rname.com", "host\tname.com",
		"..", ".", "---", "123.456.789.000",
	}

	ports := []string{
		"", ":80", ":443", ":8080", ":0", ":65535", ":abc", ":-1", ":",
	}

	paths := []string{
		"", "/", "/path", "/a/b/c", "/%20space", "/über/café", "/?query=1&b=2", "/#section",
		"/?q=🚀", "/path\\evil",
	}

	for i := 0; i < 2000; i++ {
		scheme := schemes[rng.Intn(len(schemes))]
		user := users[rng.Intn(len(users))]
		host := hosts[rng.Intn(len(hosts))]
		port := ports[rng.Intn(len(ports))]
		path := paths[rng.Intn(len(paths))]

		candidate := scheme + user + host + port + path

		func() {
			defer func() {
				if r := recover(); r != nil {
					t.Fatalf("CRITICAL PANIC on generated candidate %q: %v", candidate, r)
				}
			}()

			norm, parsed, err := normalizeAndValidateURL(candidate)
			if err == nil {
				// Valid invariants:
				if parsed == nil {
					t.Fatalf("nil *url.URL for valid candidate %q", candidate)
				}
				if parsed.Host == "" {
					t.Fatalf("empty parsed.Host for valid candidate %q", candidate)
				}
				if strings.ContainsAny(parsed.Host, " \t\r\n") {
					t.Fatalf("parsed.Host contains whitespace for valid candidate %q: %q", candidate, parsed.Host)
				}
				if !strings.HasPrefix(norm, "http://") && !strings.HasPrefix(norm, "https://") {
					t.Fatalf("missing http/https scheme for valid candidate %q: %q", candidate, norm)
				}
				// Verify hostname validity: must either contain '.' or be localhost or be valid IP
				hn := parsed.Hostname()
				if hn == "" {
					hn = parsed.Host
				}
				if !strings.Contains(hn, ".") && hn != "localhost" && net.ParseIP(hn) == nil {
					t.Fatalf("invalid hostname %q passed validation for candidate %q", hn, candidate)
				}
			}
		}()
	}
}

// TestChallengerStressConcurrentURLAndTruncate executes truncateRunes and normalizeAndValidateURL
// concurrently across 50 goroutines to guarantee complete absence of race conditions.
func TestChallengerStressConcurrentURLAndTruncate(t *testing.T) {
	const goroutines = 50
	const iterations = 200

	var wg sync.WaitGroup
	wg.Add(goroutines * 2)

	// Concurrently test truncateRunes
	for g := 0; g < goroutines; g++ {
		go func(id int) {
			defer wg.Done()
			inputs := []string{
				"⚠️ Alert! 🚀 Launching... 🍕 Done!",
				"👨‍👩‍👧‍👦 Family with 🏳️‍🌈 pride",
				"你好世界，欢迎使用 Chameleon 爬虫！",
				"\x1b[31;1mANSI text\x1b[0m with emojis 💎🦄",
				"1️⃣ Step one: 🧑🏽‍💻 Coding",
			}
			for i := 0; i < iterations; i++ {
				s := inputs[(id+i)%len(inputs)]
				maxLen := (id + i) % 25
				res := truncateRunes(s, maxLen)
				w := lipgloss.Width(res)
				if maxLen > 0 && w > maxLen {
					t.Errorf("Concurrent truncate visual width overflow: %d > %d", w, maxLen)
				}
			}
		}(g)
	}

	// Concurrently test normalizeAndValidateURL
	for g := 0; g < goroutines; g++ {
		go func(id int) {
			defer wg.Done()
			candidates := []string{
				"example.com",
				"https://example.com/test",
				"http://[::1]:8080",
				"[::1]:8080",
				"javascript:alert(1)",
				"data:text/html,test",
				"file:///etc/passwd",
				"localhost:3000",
				"user:pass@example.com",
				"http://192.168.1.1:8080/api",
			}
			for i := 0; i < iterations; i++ {
				c := candidates[(id+i)%len(candidates)]
				_, _, _ = normalizeAndValidateURL(c)
			}
		}(g)
	}

	wg.Wait()
}
