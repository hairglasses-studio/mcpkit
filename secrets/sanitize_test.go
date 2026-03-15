package secrets

import (
	"encoding/json"
	"testing"
)

// TestIsSensitiveKey tests the IsSensitiveKey function with various key names.
func TestIsSensitiveKey(t *testing.T) {
	tests := []struct {
		key      string
		expected bool
	}{
		// Should return true — sensitive keys
		{"password", true},
		{"PASSWORD", true},
		{"Password", true},
		{"user_password", true},
		{"apikey", true},
		{"api_key", true},
		{"api-key", true},
		{"APIKEY", true},
		{"token", true},
		{"access_token", true},
		{"refresh_token", true},
		{"auth_token", true},
		{"TOKEN", true},
		{"secret", true},
		{"SECRET", true},
		{"my_secret", true},
		{"credential", true},
		{"credentials", true},
		{"bearer", true},
		{"jwt", true},
		{"session", true},
		{"session_id", true},
		{"cookie", true},
		{"oauth", true},
		{"private", true},
		{"private_key", true},
		{"cert", true},
		{"certificate", true},
		{"pem", true},
		{"ssh_key", true},
		{"rsa_key", true},
		{"hmac", true},
		{"pin", true},
		{"otp", true},
		{"2fa", true},
		{"mfa", true},
		{"totp", true},
		{"seed", true},
		{"salt", true},
		{"hash", true},
		{"signing", true},
		{"encryption", true},
		{"decrypt", true},
		{"access", true},
		{"refresh", true},
		// Should return false — non-sensitive keys
		{"username", false},
		{"hostname", false},
		{"url", false},
		{"name", false},
		{"email", false},
		{"phone", false},
		{"address", false},
		{"city", false},
		{"country", false},
		{"description", false},
		{"title", false},
		{"content", false},
		{"message", false},
		{"status", false},
		{"type", false},
		{"id", false},
		{"", false},
	}

	for _, tt := range tests {
		t.Run(tt.key, func(t *testing.T) {
			got := IsSensitiveKey(tt.key)
			if got != tt.expected {
				t.Errorf("IsSensitiveKey(%q) = %v, want %v", tt.key, got, tt.expected)
			}
		})
	}
}

// TestMaskValue tests the MaskValue function across all length buckets.
func TestMaskValue(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		// Empty string
		{"empty", "", ""},
		// Short: len <= 4 → "****"
		{"len1", "a", "****"},
		{"len2", "ab", "****"},
		{"len3", "abc", "****"},
		{"len4", "abcd", "****"},
		// Medium: len <= 8 → first + "****" + last
		{"len5", "abcde", "a****e"},
		{"len6", "abcdef", "a****f"},
		{"len7", "abcdefg", "a****g"},
		{"len8", "abcdefgh", "a****h"},
		// Long: len > 8 → first2 + "****" + last2
		{"len9", "abcdefghi", "ab****hi"},
		{"len10", "abcdefghij", "ab****ij"},
		{"len16", "abcdefghijklmnop", "ab****op"},
		{"len32", "abcdefghijklmnopqrstuvwxyz012345", "ab****45"},
		// Edge: boundary at exactly 8
		{"exactly8", "12345678", "1****8"},
		// Edge: boundary at exactly 9 (first long)
		{"exactly9", "123456789", "12****89"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := MaskValue(tt.input)
			if got != tt.expected {
				t.Errorf("MaskValue(%q) = %q, want %q", tt.input, got, tt.expected)
			}
		})
	}
}

// TestSanitize tests the Sanitize function for nested maps.
func TestSanitize(t *testing.T) {
	t.Run("sensitive_string_values_are_masked", func(t *testing.T) {
		params := map[string]any{
			"password": "mysecretpass",
			"username": "alice",
		}
		result := Sanitize(params)
		if result["username"] != "alice" {
			t.Errorf("non-sensitive key changed: got %v", result["username"])
		}
		masked, ok := result["password"].(string)
		if !ok {
			t.Fatalf("password should be a string, got %T", result["password"])
		}
		if masked == "mysecretpass" {
			t.Error("password should be masked, but got plaintext")
		}
	})

	t.Run("nested_map_recursively_sanitized", func(t *testing.T) {
		params := map[string]any{
			"config": map[string]any{
				"api_key": "my-api-key-12345",
				"host":    "example.com",
			},
		}
		result := Sanitize(params)
		nested, ok := result["config"].(map[string]any)
		if !ok {
			t.Fatalf("config should be a map, got %T", result["config"])
		}
		if nested["host"] != "example.com" {
			t.Errorf("non-sensitive nested key changed: got %v", nested["host"])
		}
		maskedKey, ok := nested["api_key"].(string)
		if !ok {
			t.Fatalf("api_key should be a string, got %T", nested["api_key"])
		}
		if maskedKey == "my-api-key-12345" {
			t.Error("api_key should be masked, but got plaintext")
		}
	})

	t.Run("non_sensitive_keys_pass_through", func(t *testing.T) {
		params := map[string]any{
			"name":    "test",
			"count":   42,
			"enabled": true,
		}
		result := Sanitize(params)
		if result["name"] != "test" {
			t.Errorf("name should be unchanged, got %v", result["name"])
		}
		if result["count"] != 42 {
			t.Errorf("count should be unchanged, got %v", result["count"])
		}
		if result["enabled"] != true {
			t.Errorf("enabled should be unchanged, got %v", result["enabled"])
		}
	})

	t.Run("empty_map", func(t *testing.T) {
		result := Sanitize(map[string]any{})
		if len(result) != 0 {
			t.Errorf("empty map should produce empty result, got %v", result)
		}
	})

	t.Run("deeply_nested_sensitive_key", func(t *testing.T) {
		params := map[string]any{
			"outer": map[string]any{
				"inner": map[string]any{
					"token": "secret-token-value",
				},
			},
		}
		result := Sanitize(params)
		outer := result["outer"].(map[string]any)
		inner := outer["inner"].(map[string]any)
		maskedToken, ok := inner["token"].(string)
		if !ok {
			t.Fatalf("token should be a string, got %T", inner["token"])
		}
		if maskedToken == "secret-token-value" {
			t.Error("deeply nested token should be masked")
		}
	})
}

// TestSanitizeValue tests the sanitizeValue function directly for edge cases.
func TestSanitizeValue(t *testing.T) {
	t.Run("bytes_sensitive_key", func(t *testing.T) {
		result := sanitizeValue("password", []byte("bytesecret"))
		s, ok := result.(string)
		if !ok {
			t.Fatalf("[]byte sensitive should return string, got %T", result)
		}
		if s == "bytesecret" {
			t.Error("[]byte sensitive should be masked")
		}
	})

	t.Run("int_sensitive_key_returns_redacted", func(t *testing.T) {
		result := sanitizeValue("token", 12345)
		if result != "[REDACTED]" {
			t.Errorf("int sensitive should return [REDACTED], got %v", result)
		}
	})

	t.Run("bool_sensitive_key_returns_redacted", func(t *testing.T) {
		result := sanitizeValue("secret", true)
		if result != "[REDACTED]" {
			t.Errorf("bool sensitive should return [REDACTED], got %v", result)
		}
	})

	t.Run("nil_sensitive_key_returns_redacted", func(t *testing.T) {
		result := sanitizeValue("password", nil)
		if result != "[REDACTED]" {
			t.Errorf("nil sensitive should return [REDACTED], got %v", result)
		}
	})

	t.Run("slice_with_maps_sanitized", func(t *testing.T) {
		slice := []any{
			map[string]any{"token": "abc123456", "name": "alice"},
			map[string]any{"host": "example.com"},
		}
		result := sanitizeValue("items", slice)
		resultSlice, ok := result.([]any)
		if !ok {
			t.Fatalf("should return []any, got %T", result)
		}
		if len(resultSlice) != 2 {
			t.Fatalf("should have 2 elements, got %d", len(resultSlice))
		}
		firstMap, ok := resultSlice[0].(map[string]any)
		if !ok {
			t.Fatalf("first element should be map, got %T", resultSlice[0])
		}
		if firstMap["name"] != "alice" {
			t.Errorf("non-sensitive key should pass through, got %v", firstMap["name"])
		}
		maskedToken, ok := firstMap["token"].(string)
		if !ok {
			t.Fatalf("token should be string, got %T", firstMap["token"])
		}
		if maskedToken == "abc123456" {
			t.Error("token in slice map should be masked")
		}
	})

	t.Run("slice_with_non_map_elements_passed_through", func(t *testing.T) {
		slice := []any{"hello", 42, true}
		result := sanitizeValue("items", slice)
		resultSlice, ok := result.([]any)
		if !ok {
			t.Fatalf("should return []any, got %T", result)
		}
		if resultSlice[0] != "hello" || resultSlice[1] != 42 || resultSlice[2] != true {
			t.Errorf("non-map slice elements should pass through, got %v", resultSlice)
		}
	})

	t.Run("non_sensitive_key_with_map_value", func(t *testing.T) {
		inner := map[string]any{"password": "secret123"}
		result := sanitizeValue("config", inner)
		resultMap, ok := result.(map[string]any)
		if !ok {
			t.Fatalf("should return map, got %T", result)
		}
		if resultMap["password"] == "secret123" {
			t.Error("nested sensitive key should be masked even under non-sensitive parent")
		}
	})

	t.Run("non_sensitive_primitive", func(t *testing.T) {
		result := sanitizeValue("count", 99)
		if result != 99 {
			t.Errorf("non-sensitive int should pass through, got %v", result)
		}
	})
}

// TestSanitizeString tests URL password, Bearer token, api_key= and token= masking.
func TestSanitizeString(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		wantMask bool
		contains string
	}{
		{
			name:     "url_with_password",
			input:    "postgres://user:mysecretpassword@localhost:5432/db",
			wantMask: true,
			contains: "****",
		},
		{
			name:     "bearer_token_lowercase",
			input:    "Authorization: bearer eyJhbGciOiJSUzI1NiJ9.eyJzdWIiOiJ1c2VyIn0.sig",
			wantMask: true,
			contains: "bearer ****",
		},
		{
			name:     "bearer_token_uppercase",
			input:    "Authorization: Bearer eyJhbGciOiJSUzI1NiJ9.payload.sig",
			wantMask: true,
			contains: "Bearer ****",
		},
		{
			name:     "api_key_param",
			input:    "https://api.example.com/v1/search?api_key=super-secret-key&q=hello",
			wantMask: true,
			contains: "api_key=****",
		},
		{
			name:     "apikey_param",
			input:    "https://api.example.com/v1/search?apikey=another-secret&q=hello",
			wantMask: true,
			contains: "apikey=****",
		},
		{
			name:     "token_param",
			input:    "https://api.example.com/v1/search?token=my-token-value&q=hello",
			wantMask: true,
			contains: "token=****",
		},
		{
			name:     "token_param_mixed_case",
			input:    "https://api.example.com?Token=secret123",
			wantMask: true,
			contains: "Token=****",
		},
		{
			name:     "plain_string_unchanged",
			input:    "hello world, no secrets here",
			wantMask: false,
			contains: "hello world, no secrets here",
		},
		{
			name:     "empty_string",
			input:    "",
			wantMask: false,
			contains: "",
		},
		{
			name:     "url_without_password",
			input:    "https://example.com/path?q=hello",
			wantMask: false,
			contains: "https://example.com/path?q=hello",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := SanitizeString(tt.input)
			if tt.wantMask && got == tt.input {
				t.Errorf("SanitizeString(%q) should mask sensitive data, but returned unchanged", tt.input)
			}
			if tt.contains != "" && !containsSubstring(got, tt.contains) {
				t.Errorf("SanitizeString(%q) = %q, want it to contain %q", tt.input, got, tt.contains)
			}
			if !tt.wantMask && got != tt.input {
				t.Errorf("SanitizeString(%q) = %q, want unchanged", tt.input, got)
			}
		})
	}
}

// containsSubstring checks whether s contains substr (case-sensitive).
func containsSubstring(s, substr string) bool {
	return len(substr) == 0 || (len(s) >= len(substr) && func() bool {
		for i := 0; i <= len(s)-len(substr); i++ {
			if s[i:i+len(substr)] == substr {
				return true
			}
		}
		return false
	}())
}

// TestSanitizeHeaders tests header masking for sensitive and non-sensitive headers.
func TestSanitizeHeaders(t *testing.T) {
	tests := []struct {
		name         string
		headers      map[string]string
		maskedKeys   []string
		passedKeys   []string
	}{
		{
			name: "authorization_masked",
			headers: map[string]string{
				"Authorization": "Bearer eyJhbGciOiJSUzI1NiJ9.payload.sig",
				"Content-Type":  "application/json",
			},
			maskedKeys: []string{"Authorization"},
			passedKeys: []string{"Content-Type"},
		},
		{
			name: "x_api_key_masked",
			headers: map[string]string{
				"X-Api-Key":  "my-api-key-value-12345",
				"User-Agent": "mcpkit/1.0",
			},
			maskedKeys: []string{"X-Api-Key"},
			passedKeys: []string{"User-Agent"},
		},
		{
			name: "cookie_masked",
			headers: map[string]string{
				"Cookie":       "session=abc123; user=alice",
				"Content-Type": "text/html",
			},
			maskedKeys: []string{"Cookie"},
			passedKeys: []string{"Content-Type"},
		},
		{
			name: "x_auth_token_masked",
			headers: map[string]string{
				"X-Auth-Token": "token-value-abcdefghij",
				"Accept":       "application/json",
			},
			maskedKeys: []string{"X-Auth-Token"},
			passedKeys: []string{"Accept"},
		},
		{
			name: "set_cookie_masked",
			headers: map[string]string{
				"Set-Cookie": "session=xyz789; Path=/; HttpOnly",
				"X-Request-ID": "req-123",
			},
			maskedKeys: []string{"Set-Cookie"},
			passedKeys: []string{"X-Request-ID"},
		},
		{
			name: "x_csrf_token_masked",
			headers: map[string]string{
				"X-Csrf-Token": "csrf-token-value-12345",
				"Referer":      "https://example.com",
			},
			maskedKeys: []string{"X-Csrf-Token"},
			passedKeys: []string{"Referer"},
		},
		{
			name: "non_sensitive_headers_pass_through",
			headers: map[string]string{
				"Content-Type":   "application/json",
				"Accept":         "application/json",
				"X-Request-ID":   "req-abc-123",
			},
			maskedKeys: []string{},
			passedKeys: []string{"Content-Type", "Accept", "X-Request-ID"},
		},
		{
			name:         "empty_headers",
			headers:      map[string]string{},
			maskedKeys:   []string{},
			passedKeys:   []string{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := SanitizeHeaders(tt.headers)

			// Check that sensitive headers are masked (not equal to original)
			for _, key := range tt.maskedKeys {
				original := tt.headers[key]
				got, exists := result[key]
				if !exists {
					t.Errorf("key %q should exist in result", key)
					continue
				}
				if got == original && len(original) > 4 {
					t.Errorf("header %q should be masked, but got original value %q", key, got)
				}
			}

			// Check that non-sensitive headers pass through unchanged
			for _, key := range tt.passedKeys {
				original := tt.headers[key]
				got, exists := result[key]
				if !exists {
					t.Errorf("key %q should exist in result", key)
					continue
				}
				if got != original {
					t.Errorf("header %q should be unchanged: got %q, want %q", key, got, original)
				}
			}
		})
	}

	t.Run("case_insensitive_sensitive_header_detection", func(t *testing.T) {
		// The implementation lowercases the key for comparison
		headers := map[string]string{
			"authorization": "Bearer token123456789",
			"COOKIE":        "session=abcdefghij",
		}
		result := SanitizeHeaders(headers)
		if result["authorization"] == "Bearer token123456789" {
			t.Error("lowercase authorization should be masked")
		}
		if result["COOKIE"] == "session=abcdefghij" {
			t.Error("uppercase COOKIE should be masked")
		}
	})
}

// TestRedactedString tests the RedactedString type.
func TestRedactedString(t *testing.T) {
	t.Run("String_returns_masked", func(t *testing.T) {
		r := RedactedString("mysecretvalue123")
		masked := r.String()
		if masked == "mysecretvalue123" {
			t.Error("String() should return masked value, not plaintext")
		}
		// Should be in "first2****last2" format (len > 8)
		expected := MaskValue("mysecretvalue123")
		if masked != expected {
			t.Errorf("String() = %q, want %q", masked, expected)
		}
	})

	t.Run("Value_returns_raw", func(t *testing.T) {
		r := RedactedString("mysecretvalue123")
		if r.Value() != "mysecretvalue123" {
			t.Errorf("Value() should return raw string, got %q", r.Value())
		}
	})

	t.Run("MarshalJSON_returns_masked", func(t *testing.T) {
		r := RedactedString("mysecretvalue123")
		data, err := r.MarshalJSON()
		if err != nil {
			t.Fatalf("MarshalJSON() error: %v", err)
		}
		var s string
		if err := json.Unmarshal(data, &s); err != nil {
			t.Fatalf("MarshalJSON() produced invalid JSON: %v", err)
		}
		if s == "mysecretvalue123" {
			t.Error("MarshalJSON() should produce masked value, not plaintext")
		}
		expected := MaskValue("mysecretvalue123")
		if s != expected {
			t.Errorf("MarshalJSON() decoded = %q, want %q", s, expected)
		}
	})

	t.Run("short_value_masked_as_stars", func(t *testing.T) {
		r := RedactedString("hi")
		if r.String() != "****" {
			t.Errorf("short String() should be ****, got %q", r.String())
		}
		data, err := r.MarshalJSON()
		if err != nil {
			t.Fatalf("MarshalJSON() error: %v", err)
		}
		var s string
		json.Unmarshal(data, &s)
		if s != "****" {
			t.Errorf("short MarshalJSON() should be ****, got %q", s)
		}
	})

	t.Run("empty_value", func(t *testing.T) {
		r := RedactedString("")
		if r.String() != "" {
			t.Errorf("empty String() should be empty, got %q", r.String())
		}
		if r.Value() != "" {
			t.Errorf("empty Value() should be empty, got %q", r.Value())
		}
		data, err := r.MarshalJSON()
		if err != nil {
			t.Fatalf("MarshalJSON() error: %v", err)
		}
		var s string
		json.Unmarshal(data, &s)
		if s != "" {
			t.Errorf("empty MarshalJSON() should decode to empty, got %q", s)
		}
	})

	t.Run("json_encoding_in_struct", func(t *testing.T) {
		type Config struct {
			Name     string         `json:"name"`
			APIKey   RedactedString `json:"api_key"`
		}
		cfg := Config{Name: "test", APIKey: RedactedString("supersecretkey123")}
		data, err := json.Marshal(cfg)
		if err != nil {
			t.Fatalf("json.Marshal error: %v", err)
		}
		s := string(data)
		if containsSubstring(s, "supersecretkey123") {
			t.Error("JSON output should not contain plaintext secret")
		}
		if !containsSubstring(s, "****") {
			t.Error("JSON output should contain masked value with ****")
		}
	})
}

// TestSecureCompare tests constant-time string comparison.
func TestSecureCompare(t *testing.T) {
	tests := []struct {
		name     string
		a        string
		b        string
		expected bool
	}{
		{"equal_strings", "hello", "hello", true},
		{"equal_empty", "", "", true},
		{"unequal_strings", "hello", "world", false},
		{"different_lengths_ab", "hello", "hi", false},
		{"different_lengths_ba", "hi", "hello", false},
		{"equal_long", "abcdefghijklmnopqrstuvwxyz", "abcdefghijklmnopqrstuvwxyz", true},
		{"one_char_diff", "password1", "password2", false},
		{"equal_single_char", "x", "x", true},
		{"unequal_single_char", "x", "y", false},
		{"empty_vs_nonempty", "", "a", false},
		{"nonempty_vs_empty", "a", "", false},
		{"equal_with_special_chars", "p@ssw0rd!#$%", "p@ssw0rd!#$%", true},
		{"unequal_with_special_chars", "p@ssw0rd!#$%", "p@ssw0rd!#$&", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := SecureCompare(tt.a, tt.b)
			if got != tt.expected {
				t.Errorf("SecureCompare(%q, %q) = %v, want %v", tt.a, tt.b, got, tt.expected)
			}
		})
	}
}

// TestSanitizeString_URLEdgeCases tests additional URL masking scenarios.
func TestSanitizeString_URLEdgeCases(t *testing.T) {
	t.Run("multiple_patterns_in_one_string", func(t *testing.T) {
		// URL with password AND api_key param
		input := "postgres://admin:secret123@db.example.com?api_key=key456"
		got := SanitizeString(input)
		if containsSubstring(got, "secret123") {
			t.Error("URL password should be masked")
		}
		if containsSubstring(got, "key456") {
			t.Error("api_key value should be masked")
		}
	})

	t.Run("token_at_end_of_url", func(t *testing.T) {
		input := "https://api.example.com/data?token=mytokenvalue"
		got := SanitizeString(input)
		if containsSubstring(got, "mytokenvalue") {
			t.Error("token value should be masked")
		}
	})

	t.Run("api_hyphen_key_param", func(t *testing.T) {
		input := "https://api.example.com?api-key=hyphenkey123"
		got := SanitizeString(input)
		if containsSubstring(got, "hyphenkey123") {
			t.Error("api-key value should be masked")
		}
	})
}

// TestSanitize_SensitivePatterns ensures all sensitive pattern types are covered.
func TestSanitize_SensitivePatterns(t *testing.T) {
	sensitiveKeys := []string{
		"password", "passwd", "secret", "token", "key", "credential",
		"cred", "auth", "bearer", "api_key", "private", "oauth",
		"jwt", "session", "cookie", "cert", "pem", "signing",
		"encryption", "salt", "hash", "hmac", "pin", "otp", "seed",
	}

	for _, key := range sensitiveKeys {
		t.Run("masks_"+key, func(t *testing.T) {
			params := map[string]any{key: "plaintextvalue123"}
			result := Sanitize(params)
			val, ok := result[key].(string)
			if !ok {
				t.Fatalf("result[%q] should be string, got %T", key, result[key])
			}
			if val == "plaintextvalue123" {
				t.Errorf("key %q should be masked, got plaintext", key)
			}
		})
	}
}
