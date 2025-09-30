package domain

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"net/url"
	"strings"
)

type (
	NormalizedURL struct {
		value string
	}

	ContentHash struct {
		value string
	}
)

func NewNormalizedURL(rawURL string) (*NormalizedURL, error) {
	parsedURL, err := url.Parse(rawURL)
	if err != nil {

		return nil, fmt.Errorf("failed to parse URL: %w", err)
	}

	if parsedURL.Scheme == "" {
		parsedURL.Scheme = "https"
	}

	parsedURL.Scheme = strings.ToLower(parsedURL.Scheme)
	parsedURL.Host = strings.ToLower(parsedURL.Host)

	if (parsedURL.Scheme == "http" && strings.HasSuffix(parsedURL.Host, ":80")) ||
		(parsedURL.Scheme == "https" && strings.HasSuffix(parsedURL.Host, ":443")) {
		parsedURL.Host = parsedURL.Host[:strings.LastIndex(parsedURL.Host, ":")]
	}

	if parsedURL.Path == "/" {
		parsedURL.Path = ""
	}

	parsedURL.Fragment = ""

	return &NormalizedURL{value: parsedURL.String()}, nil
}

func (u *NormalizedURL) String() string {

	return u.value
}

func NewContentHash(content string) *ContentHash {
	hash := sha256.Sum256([]byte(content))

	return &ContentHash{value: hex.EncodeToString(hash[:])}
}

func (h *ContentHash) String() string {

	return h.value
}
