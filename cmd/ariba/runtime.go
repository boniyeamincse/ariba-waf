package main

import (
	"errors"
	"fmt"
	"net"
	"net/http"
	"strconv"
	"time"
)

func validateListenAddress(address string, allowRemote bool) (string, error) {
	host, port, err := net.SplitHostPort(address)
	if err != nil {
		return "", fmt.Errorf("invalid listen address: %w", err)
	}
	ip := net.ParseIP(host)
	if ip == nil {
		return "", errors.New("listen address must use an IP literal")
	}
	n, err := strconv.Atoi(port)
	if err != nil || n < 1 || n > 65535 {
		return "", errors.New("listen port must be between 1 and 65535")
	}
	if !ip.IsLoopback() && !allowRemote {
		return "", errors.New("non-loopback listen address requires -allow-remote-listen")
	}
	return net.JoinHostPort(ip.String(), strconv.Itoa(n)), nil
}

// probeLiveness works in a shell-free image. It ignores proxy environment
// variables and does not follow redirects or initialize persistent state.
func probeLiveness(url string) error {
	transport := &http.Transport{Proxy: nil}
	defer transport.CloseIdleConnections()
	client := &http.Client{Timeout: 2 * time.Second, Transport: transport, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}
	response, err := client.Get(url)
	if err != nil {
		return err
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		return fmt.Errorf("liveness returned HTTP %d", response.StatusCode)
	}
	return nil
}
