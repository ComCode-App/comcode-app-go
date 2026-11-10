package config

import (
	"fmt"
	"strings"
)

// reviewCodeLen is the length of a fixed App Store / Play review sign-in code: the same
// 6 digits the email-code form accepts (TEC-1182).
const reviewCodeLen = 6

// weakReviewCodes are well-known 6-digit codes that the pattern checks below do not catch.
var weakReviewCodes = map[string]bool{
	"112233": true, "102030": true, "147258": true, "258369": true, "159753": true,
	"147852": true, "789456": true, "456789": true, "000123": true, "111222": true,
	"101010": true, "202020": true, "696969": true, "131313": true, "123321": true,
	"654456": true, "100000": true, "999000": true,
}

// parseReviewAccounts parses AUTH_REVIEW_ACCOUNTS ("a@x.com:246810,b@y.com:913572") into a
// lower-cased email -> code map. Blank means disabled (empty map). Unlike most optional knobs
// a malformed value is an error, never silently ignored: a half-parsed allowlist would either
// lock the store reviewer out or leave a weak bypass in production.
//
// Errors name the 1-based entry, never the code (the value is a secret).
func parseReviewAccounts(raw string) (map[string]string, error) {
	out := map[string]string{}
	if strings.TrimSpace(raw) == "" {
		return out, nil
	}
	for i, item := range strings.Split(raw, ",") {
		n := i + 1
		item = strings.TrimSpace(item)
		if item == "" {
			return nil, fmt.Errorf("config: AUTH_REVIEW_ACCOUNTS entry %d is empty (format email:code[,email:code])", n)
		}
		email, code, ok := strings.Cut(item, ":")
		email = strings.ToLower(strings.TrimSpace(email))
		code = strings.TrimSpace(code)
		if !ok {
			return nil, fmt.Errorf("config: AUTH_REVIEW_ACCOUNTS entry %d has no code (format email:code)", n)
		}
		if !validReviewEmail(email) {
			return nil, fmt.Errorf("config: AUTH_REVIEW_ACCOUNTS entry %d has an invalid email", n)
		}
		if err := checkReviewCode(code); err != nil {
			return nil, fmt.Errorf("config: AUTH_REVIEW_ACCOUNTS entry %d: %w", n, err)
		}
		if _, dup := out[email]; dup {
			return nil, fmt.Errorf("config: AUTH_REVIEW_ACCOUNTS entry %d repeats an email", n)
		}
		out[email] = code
	}
	return out, nil
}

func validReviewEmail(email string) bool {
	local, domain, ok := strings.Cut(email, "@")
	return ok && local != "" && strings.Contains(domain, ".") &&
		!strings.HasPrefix(domain, ".") && !strings.HasSuffix(domain, ".") &&
		!strings.ContainsAny(email, " \t") && !strings.Contains(domain, "@")
}

// checkReviewCode requires exactly 6 digits that are not trivially guessable.
func checkReviewCode(code string) error {
	if len(code) != reviewCodeLen {
		return fmt.Errorf("the code must be exactly %d digits", reviewCodeLen)
	}
	for _, r := range code {
		if r < '0' || r > '9' {
			return fmt.Errorf("the code must be exactly %d digits", reviewCodeLen)
		}
	}
	if weakReviewCode(code) {
		return fmt.Errorf("the code is too easy to guess (repeated, sequential or a common pattern); pick 6 random digits")
	}
	return nil
}

// weakReviewCode reports repeating patterns (000000, 121212, 123123), runs that step by one in
// either direction (123456, 654321, 890123) and the common codes in weakReviewCodes.
func weakReviewCode(code string) bool {
	if weakReviewCodes[code] {
		return true
	}
	for _, period := range []int{1, 2, 3} {
		if strings.Repeat(code[:period], reviewCodeLen/period) == code {
			return true
		}
	}
	up, down := true, true
	for i := 1; i < len(code); i++ {
		d := (int(code[i]) - int(code[i-1]) + 10) % 10
		up = up && d == 1
		down = down && d == 9
	}
	return up || down
}
