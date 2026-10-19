//go:build darwin

// Package keychain uses Security.framework directly; credentials never enter files or argv.
package keychain

import (
	"context"
	"errors"
	"github.com/ebitengine/purego"
	"strings"
	"sync"
	"unsafe"
)

type Store struct{}

var native struct {
	once             sync.Once
	err              error
	createDictionary func(uintptr, int, uintptr, uintptr) uintptr
	createString     func(uintptr, string, uint32) uintptr
	set              func(uintptr, uintptr, uintptr)
	createData       func(uintptr, []byte, int) uintptr
	dataLength       func(uintptr) int
	dataBytes        func(uintptr) unsafe.Pointer
	symbol           func(uintptr, string) unsafe.Pointer
	release          func(uintptr)
	copyMatching     func(uintptr, *uintptr) int32
	update           func(uintptr, uintptr) int32
	add              func(uintptr, *uintptr) int32
	delete           func(uintptr) int32
	keys             map[string]uintptr
	callbacks        [2]uintptr
}

func initialize() error {
	native.once.Do(func() {
		cf, e := purego.Dlopen("/System/Library/Frameworks/CoreFoundation.framework/CoreFoundation", purego.RTLD_NOW|purego.RTLD_LOCAL)
		if e != nil {
			native.err = errors.New("CoreFoundation unavailable")
			return
		}
		sec, e := purego.Dlopen("/System/Library/Frameworks/Security.framework/Security", purego.RTLD_NOW|purego.RTLD_LOCAL)
		if e != nil {
			native.err = errors.New("Security framework unavailable")
			return
		}
		purego.RegisterLibFunc(&native.createDictionary, cf, "CFDictionaryCreateMutable")
		purego.RegisterLibFunc(&native.createString, cf, "CFStringCreateWithCString")
		purego.RegisterLibFunc(&native.set, cf, "CFDictionarySetValue")
		purego.RegisterLibFunc(&native.createData, cf, "CFDataCreate")
		purego.RegisterLibFunc(&native.dataLength, cf, "CFDataGetLength")
		purego.RegisterLibFunc(&native.dataBytes, cf, "CFDataGetBytePtr")
		purego.RegisterLibFunc(&native.release, cf, "CFRelease")
		purego.RegisterLibFunc(&native.copyMatching, sec, "SecItemCopyMatching")
		purego.RegisterLibFunc(&native.update, sec, "SecItemUpdate")
		purego.RegisterLibFunc(&native.add, sec, "SecItemAdd")
		purego.RegisterLibFunc(&native.delete, sec, "SecItemDelete")
		purego.RegisterLibFunc(&native.symbol, purego.RTLD_DEFAULT, "dlsym")
		native.keys = map[string]uintptr{}
		for _, name := range []string{"kSecClass", "kSecClassGenericPassword", "kSecAttrService", "kSecAttrAccount", "kSecValueData", "kSecReturnData", "kSecMatchLimit", "kSecMatchLimitOne"} {
			sym := native.symbol(sec, name)
			if sym == nil {
				native.err = errors.New("Security constant unavailable")
				return
			}
			native.keys[name] = *(*uintptr)(sym)
		}
		sym := native.symbol(cf, "kCFBooleanTrue")
		if sym == nil {
			native.err = errors.New("CoreFoundation constant unavailable")
			return
		}
		native.keys["true"] = *(*uintptr)(sym)
		for i, name := range []string{"kCFTypeDictionaryKeyCallBacks", "kCFTypeDictionaryValueCallBacks"} {
			sym, e := purego.Dlsym(cf, name)
			if e != nil {
				native.err = e
				return
			}
			native.callbacks[i] = sym
		}
	})
	return native.err
}
func dictionary() uintptr {
	return native.createDictionary(0, 0, native.callbacks[0], native.callbacks[1])
}
func query(ctx context.Context, account string) (uintptr, error) {
	if e := ctx.Err(); e != nil {
		return 0, e
	}
	if len(account) == 0 || len(account) > 2048 || strings.ContainsRune(account, 0) {
		return 0, errors.New("invalid Keychain account")
	}
	if e := initialize(); e != nil {
		return 0, e
	}
	q := dictionary()
	a := native.createString(0, account, 0x08000100)
	service := native.createString(0, "yz.sh.mygo-go-voice.refinement", 0x08000100)
	native.set(q, native.keys["kSecClass"], native.keys["kSecClassGenericPassword"])
	native.set(q, native.keys["kSecAttrService"], service)
	native.set(q, native.keys["kSecAttrAccount"], a)
	native.release(service)
	native.release(a)
	return q, nil
}
func (Store) Get(ctx context.Context, account string) (string, error) {
	q, e := query(ctx, account)
	if e != nil {
		return "", e
	}
	defer native.release(q)
	native.set(q, native.keys["kSecReturnData"], native.keys["true"])
	native.set(q, native.keys["kSecMatchLimit"], native.keys["kSecMatchLimitOne"])
	var result uintptr
	if native.copyMatching(q, &result) != 0 {
		return "", errors.New("Keychain read unavailable")
	}
	defer native.release(result)
	n := native.dataLength(result)
	if n < 1 || n > 4096 {
		return "", errors.New("invalid Keychain credential size")
	}
	return string(unsafe.Slice((*byte)(native.dataBytes(result)), n)), nil
}
func (Store) Set(ctx context.Context, account, value string) error {
	if len(value) == 0 || len(value) > 4096 || strings.ContainsRune(value, 0) {
		return errors.New("invalid credential")
	}
	q, e := query(ctx, account)
	if e != nil {
		return e
	}
	defer native.release(q)
	b := []byte(value)
	defer clear(b)
	data := native.createData(0, b, len(b))
	defer native.release(data)
	attrs := dictionary()
	defer native.release(attrs)
	native.set(attrs, native.keys["kSecValueData"], data)
	status := native.update(q, attrs)
	if status == -25300 {
		native.set(q, native.keys["kSecValueData"], data)
		status = native.add(q, nil)
	}
	if status != 0 {
		return errors.New("Keychain write unavailable")
	}
	return nil
}
func (Store) Delete(ctx context.Context, account string) error {
	q, e := query(ctx, account)
	if e != nil {
		return e
	}
	defer native.release(q)
	status := native.delete(q)
	if status != 0 && status != -25300 {
		return errors.New("Keychain delete unavailable")
	}
	return nil
}
