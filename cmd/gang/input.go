package main

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"io"
	"strings"
)

const maximumMessageBytes = 1 << 20

func readBody(reader io.Reader) (string, error) {
	limited := io.LimitReader(reader, maximumMessageBytes+1)
	data, err := io.ReadAll(limited)
	if err != nil {
		return "", fmt.Errorf("read message: %w", err)
	}
	if len(data) > maximumMessageBytes {
		return "", refuseError("message is larger than %d bytes", maximumMessageBytes)
	}
	body := strings.TrimSuffix(string(data), "\n")
	if body == "" {
		return "", refuseError("message body is empty")
	}
	return body, nil
}

func randomID(prefix string) (string, error) {
	var random [16]byte
	if _, err := rand.Read(random[:]); err != nil {
		return "", fmt.Errorf("create %s id: %w", prefix, err)
	}
	return prefix + "-" + hex.EncodeToString(random[:]), nil
}
