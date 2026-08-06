package common

import (
	"fmt"
	"net/url"
	"strings"
)

// Eliminating whitspace then replacing it with '+' symbol
// Example:
// Seishun no -> Seishun+no,
// made in abyss -> made+in+abyss
func StringToQueryFormat(rawInput string) string {
	rawSplitted := strings.Split(rawInput, " ")
	if len(rawSplitted) == 0 {
		return rawInput
	}

	return strings.Join(rawSplitted, "+")
}

// Getting base url from any url
// Example:
// https://youtube.com, https://yandex.ru
func GetBaseURL(rawUrl string) (string, error) {
	parsedURL, err := url.Parse(rawUrl)
	if err != nil {
		return "", err
	}

	return fmt.Sprintf("%s://%s", parsedURL.Scheme, parsedURL.Host), nil
}

func TruncatedString(raw string, maxChar int) string {
	if len(raw) <= maxChar {
		return raw
	}

	return raw[:maxChar] + "..."
}
