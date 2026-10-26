//go:build linux

package keychain

import (
	"bytes"
	"context"
	"errors"
	"os/exec"
	"strings"
	"time"

	"github.com/godbus/dbus/v5"
)

type Store struct{}

const service = "yz.sh.mygo-go-voice.refinement"

func validAccount(account string) bool {
	return len(account) > 0 && len(account) <= 2048 && !strings.ContainsRune(account, 0)
}

// secret-tool delegates unlock prompts and encryption to the desktop Secret
// Service. A missing/locked keyring is an error, never a plaintext fallback.
func secret(ctx context.Context, action, account, value string) (string, error) {
	if !validAccount(account) {
		return "", errors.New("invalid keyring account")
	}
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	args := []string{action}
	if action == "store" {
		args = append(args, "--label=Go Voice API key")
	}
	args = append(args, "service", service, "account", account)
	cmd := exec.CommandContext(ctx, "secret-tool", args...)
	if action == "store" {
		cmd.Stdin = strings.NewReader(value)
	}
	var result bytes.Buffer
	cmd.Stdout = &result
	cmd.WaitDelay = 250 * time.Millisecond
	if err := cmd.Run(); err != nil {
		return "", errors.New("系统钥匙环不可用，请安装 libsecret-tools 并解锁登录钥匙环")
	}
	defer clear(result.Bytes())
	return strings.TrimSuffix(result.String(), "\n"), nil
}
func (Store) Get(ctx context.Context, account string) (string, error) {
	v, err := secret(ctx, "lookup", account, "")
	if err == nil && (len(v) == 0 || len(v) > 4096 || strings.ContainsRune(v, 0)) {
		return "", errors.New("invalid credential")
	}
	return v, err
}
func (Store) Set(ctx context.Context, account, value string) error {
	if len(value) == 0 || len(value) > 4096 || strings.ContainsRune(value, 0) {
		return errors.New("invalid credential")
	}
	_, err := secret(ctx, "store", account, value)
	return err
}
func (s Store) Delete(ctx context.Context, account string) error {
	has, err := s.Has(ctx, account)
	if err != nil || !has {
		return err
	}
	_, err = secret(ctx, "clear", account, "")
	return err
}

// SearchItems returns paths only: Has never reads credential contents or
// unlocks a keyring just to paint a settings-page hint.
func (Store) Has(ctx context.Context, account string) (bool, error) {
	if !validAccount(account) {
		return false, errors.New("invalid keyring account")
	}
	ctx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()
	bus, err := dbus.ConnectSessionBus()
	if err != nil {
		return false, errors.New("system keyring unavailable")
	}
	defer bus.Close()
	var unlocked, locked []dbus.ObjectPath
	err = bus.Object("org.freedesktop.secrets", "/org/freedesktop/secrets").CallWithContext(ctx, "org.freedesktop.Secret.Service.SearchItems", 0, map[string]string{"service": service, "account": account}).Store(&unlocked, &locked)
	if err != nil {
		return false, errors.New("system keyring unavailable")
	}
	return len(unlocked)+len(locked) > 0, nil
}
