package linuxhost

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"io"
	"os"
	"slices"
	"sort"
	"strings"
	"syscall"
	"time"

	"github.com/godbus/dbus/v5"
	"github.com/vfasky/my-ai-audio/gui/internal/platform/bridge"
	"github.com/vfasky/my-ai-audio/gui/internal/protocol"
)

const bluez = "org.bluez"
const adapterIF = bluez + ".Adapter1"
const deviceIF = bluez + ".Device1"
const serviceIF = bluez + ".GattService1"
const charIF = bluez + ".GattCharacteristic1"

type objects map[dbus.ObjectPath]map[string]map[string]dbus.Variant
type audioPacket struct {
	generation uint64
	data       []byte
	err        error
}
type bluetooth struct {
	ctx                                   context.Context
	bus                                   *dbus.Conn
	out                                   emitter
	adapter, selected, preferred, control dbus.ObjectPath
	notify                                *os.File
	packets                               chan audioPacket
	generation                            uint64
	minSignal                             dbus.Sequence
	assembler                             protocol.Assembler
	scanUntil                             time.Time
	nextConnect                           time.Time
}

func RunBluetooth(parent context.Context, in io.Reader, out io.Writer) error {
	ctx, cancel := context.WithCancel(parent)
	defer cancel()
	input := commands(ctx, cancel, in)
	b := &bluetooth{ctx: ctx, out: emitter{out: out}, packets: make(chan audioPacket, 256)}
	var err error
	b.bus, err = dbus.ConnectSystemBus()
	if err != nil {
		b.out.permission("bluetooth", "denied")
		b.fail("无法连接蓝牙服务，请检查 BlueZ 是否运行")
		return err
	}
	defer b.bus.Close()
	defer b.disconnect()
	signals := make(chan *dbus.Signal, 512)
	b.bus.Signal(signals)
	defer b.bus.RemoveSignal(signals)
	if err = b.bus.AddMatchSignal(dbus.WithMatchSender(bluez), dbus.WithMatchInterface("org.freedesktop.DBus.Properties"), dbus.WithMatchMember("PropertiesChanged")); err != nil {
		return err
	}
	b.out.send(bridge.Event{Type: "ready"})
	b.permissions()
	ticker := time.NewTicker(2 * time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return nil
		case c := <-input:
			switch c.Type {
			case "permissions":
				b.permissions()
			case "scan":
				b.scan()
			case "set-preferred-device":
				if c.DeviceID != "" && !validDevicePath(c.DeviceID) {
					b.fail("无效的蓝牙设备")
					continue
				}
				if b.preferred != dbus.ObjectPath(c.DeviceID) {
					b.disconnect()
				}
				b.preferred = dbus.ObjectPath(c.DeviceID)
				b.nextConnect = time.Time{}
				if b.preferred != "" && b.selected == "" {
					b.scan()
					b.connectPreferred()
				}
			case "host_ready", "host_state":
				if b.control == "" {
					continue
				}
				v := map[string]any{"v": 1, "type": c.Type}
				if c.Type == "host_state" {
					v["state"] = c.State
				}
				data, _ := json.Marshal(v)
				if _, err := protocol.ParseControl(data); err != nil {
					continue
				}
				if err := b.call(b.control, charIF+".WriteValue", data, map[string]dbus.Variant{"type": dbus.MakeVariant("request")}).Err; err != nil {
					b.fail("蓝牙控制写入失败，请重新连接")
					b.disconnect()
				}
			case "shutdown":
				return nil
			}
		case signal := <-signals:
			if signal == nil {
				return errors.New("bluez disconnected")
			}
			b.signal(signal)
		case packet := <-b.packets:
			if packet.generation != b.generation || b.notify == nil {
				continue
			}
			if packet.err != nil {
				b.fail("蓝牙音频连接已断开")
				b.disconnect()
				continue
			}
			frame, err := b.assembler.Push(packet.data)
			if err != nil {
				b.fail("蓝牙音频帧无效")
				b.disconnect()
				continue
			}
			if frame != nil {
				// Zero sequence/session IDs must be present in the wire event.
				b.audio(frame)
			}
		case now := <-ticker.C:
			if !b.scanUntil.IsZero() {
				b.discover()
				if now.After(b.scanUntil) {
					b.stopScan()
				}
			}
			if b.preferred != "" && b.selected == "" && now.After(b.nextConnect) {
				b.connectPreferred()
			}
		}
	}
}

func (b *bluetooth) audio(f *protocol.Frame) {
	v := struct {
		Type      string `json:"type"`
		SessionID uint32 `json:"sessionId"`
		Sequence  uint32 `json:"sequence"`
		Payload   string `json:"payloadBase64"`
	}{"audio", f.SessionID, f.Sequence, base64.StdEncoding.EncodeToString(f.Payload)}
	data, _ := json.Marshal(v)
	b.out.mu.Lock()
	defer b.out.mu.Unlock()
	_, _ = b.out.out.Write(append(data, '\n'))
}
func validDevicePath(s string) bool {
	return len(s) <= 256 && dbus.ObjectPath(s).IsValid() && strings.HasPrefix(s, "/org/bluez/hci") && strings.Contains(s, "/dev_")
}
func (b *bluetooth) call(path dbus.ObjectPath, method string, args ...any) *dbus.Call {
	ctx, cancel := context.WithTimeout(b.ctx, 15*time.Second)
	defer cancel()
	return b.bus.Object(bluez, path).CallWithContext(ctx, method, 0, args...)
}
func (b *bluetooth) objects() (objects, error) {
	var all objects
	err := b.call("/", "org.freedesktop.DBus.ObjectManager.GetManagedObjects").Store(&all)
	return all, err
}
func (b *bluetooth) fail(message string) { b.out.send(bridge.Event{Type: "error", Message: message}) }
func (b *bluetooth) permissions() {
	all, err := b.objects()
	if err == nil {
		for _, interfaces := range all {
			if powered, _ := interfaces[adapterIF]["Powered"].Value().(bool); powered {
				b.out.permission("bluetooth", "granted")
				return
			}
		}
	}
	b.out.permission("bluetooth", "denied")
}
func (b *bluetooth) scan() {
	all, err := b.objects()
	if err != nil {
		b.fail("无法访问蓝牙服务")
		return
	}
	var paths []string
	for path, interfaces := range all {
		if powered, _ := interfaces[adapterIF]["Powered"].Value().(bool); powered {
			paths = append(paths, string(path))
		}
	}
	sort.Strings(paths)
	if len(paths) == 0 {
		b.out.permission("bluetooth", "denied")
		b.fail("请先打开系统蓝牙")
		return
	}
	if b.adapter == "" {
		b.adapter = dbus.ObjectPath(paths[0])
	}
	filter := map[string]dbus.Variant{"Transport": dbus.MakeVariant("le"), "UUIDs": dbus.MakeVariant([]string{protocol.ServiceUUID})}
	if err := b.call(b.adapter, adapterIF+".SetDiscoveryFilter", filter).Err; err != nil {
		b.fail("无法设置蓝牙扫描")
		return
	}
	if b.scanUntil.IsZero() {
		if err := b.call(b.adapter, adapterIF+".StartDiscovery").Err; err != nil {
			b.fail("无法扫描，请检查蓝牙权限")
			return
		}
	}
	b.scanUntil = time.Now().Add(20 * time.Second)
	b.discover()
}
func voiceDevice(p map[string]dbus.Variant) bool {
	uuids, _ := p["UUIDs"].Value().([]string)
	return slices.ContainsFunc(uuids, func(s string) bool { return strings.EqualFold(s, protocol.ServiceUUID) })
}
func (b *bluetooth) discover() {
	all, err := b.objects()
	if err != nil {
		return
	}
	n := 0
	for path, interfaces := range all {
		p := interfaces[deviceIF]
		if !voiceDevice(p) || n >= 64 {
			continue
		}
		n++
		name, _ := p["Name"].Value().(string)
		if name == "" || len(name) > 128 {
			name = "Go Voice"
		}
		b.out.send(bridge.Event{Type: "discovered", DeviceID: string(path), DeviceName: name})
	}
}
func (b *bluetooth) stopScan() {
	if !b.scanUntil.IsZero() && b.adapter != "" {
		_ = b.call(b.adapter, adapterIF+".StopDiscovery").Err
	}
	b.scanUntil = time.Time{}
}

// selectCharacteristics scopes every characteristic to this device's voice
// service, never to another connected peripheral or a service with a similar name.
func selectCharacteristics(all objects, device dbus.ObjectPath) (map[string]dbus.ObjectPath, error) {
	services := map[dbus.ObjectPath]bool{}
	for path, interfaces := range all {
		p := interfaces[serviceIF]
		uuid, _ := p["UUID"].Value().(string)
		owner, _ := p["Device"].Value().(dbus.ObjectPath)
		if owner == device && strings.EqualFold(uuid, protocol.ServiceUUID) {
			services[path] = true
		}
	}
	found := map[string]dbus.ObjectPath{}
	for path, interfaces := range all {
		p := interfaces[charIF]
		service, _ := p["Service"].Value().(dbus.ObjectPath)
		if !services[service] {
			continue
		}
		uuid, _ := p["UUID"].Value().(string)
		uuid = strings.ToLower(uuid)
		flags, _ := p["Flags"].Value().([]string)
		required := map[string][]string{protocol.ControlUUID: {"write", "notify"}, protocol.AudioUUID: {"notify"}, protocol.InfoUUID: {"read"}}[uuid]
		if required == nil {
			continue
		}
		if found[uuid] != "" {
			return nil, errors.New("duplicate characteristic")
		}
		for _, flag := range required {
			if !slices.Contains(flags, flag) {
				return nil, errors.New("unsupported characteristic")
			}
		}
		found[uuid] = path
	}
	if len(found) != 3 {
		return nil, errors.New("missing voice characteristics")
	}
	return found, nil
}
func (b *bluetooth) connectPreferred() {
	if b.selected != "" {
		return
	}
	b.nextConnect = time.Now().Add(5 * time.Second)
	all, err := b.objects()
	if err != nil {
		return
	}
	if !voiceDevice(all[b.preferred][deviceIF]) {
		return
	}
	b.stopScan()
	b.selected = b.preferred
	connected, _ := all[b.selected][deviceIF]["Connected"].Value().(bool)
	if !connected {
		if err = b.call(b.selected, deviceIF+".Connect").Err; err != nil {
			b.fail("连接工牌失败，请确认工牌未被其他电脑占用")
			b.disconnect()
			return
		}
	}
	deadline := time.Now().Add(12 * time.Second)
	var chars map[string]dbus.ObjectPath
	for time.Now().Before(deadline) {
		all, err = b.objects()
		if err == nil {
			resolved, _ := all[b.selected][deviceIF]["ServicesResolved"].Value().(bool)
			if resolved {
				chars, err = selectCharacteristics(all, b.selected)
				break
			}
		}
		select {
		case <-b.ctx.Done():
			b.disconnect()
			return
		case <-time.After(150 * time.Millisecond):
		}
	}
	if err != nil || len(chars) != 3 {
		b.fail("工牌语音服务不可用")
		b.disconnect()
		return
	}
	b.control = chars[protocol.ControlUUID]
	// AcquireNotify returns the negotiated MTU without relying on private
	// BlueZ properties. It is supported for remote notify characteristics.
	var fd dbus.UnixFD
	var mtu uint16
	err = b.call(chars[protocol.AudioUUID], charIF+".AcquireNotify", map[string]dbus.Variant{}).Store(&fd, &mtu)
	if err != nil {
		b.fail("蓝牙音频订阅失败，请检查 BlueZ 版本")
		b.disconnect()
		return
	}
	// Let os.File use the poller, so Close interrupts a blocked audio read.
	if err = syscall.SetNonblock(int(fd), true); err != nil {
		_ = syscall.Close(int(fd))
		b.disconnect()
		return
	}
	b.notify = os.NewFile(uintptr(fd), "go-voice-audio")
	if mtu < 185 {
		b.fail("蓝牙 MTU 小于 185，无法接收语音")
		b.disconnect()
		return
	}
	if err = b.call(b.control, charIF+".StartNotify").Err; err != nil {
		b.fail("蓝牙控制订阅失败")
		b.disconnect()
		return
	}
	var hello []byte
	infoCall := b.call(chars[protocol.InfoUUID], charIF+".ReadValue", map[string]dbus.Variant{})
	if err = infoCall.Store(&hello); err != nil {
		b.fail("无法读取工牌信息")
		b.disconnect()
		return
	}
	v, err := protocol.ParseControl(hello)
	if err != nil || v.Type != "hello" {
		b.fail("工牌协议不兼容")
		b.disconnect()
		return
	}
	// Old notifications/disconnects can still be queued when reconnecting to
	// the same BlueZ path. No PTT is valid before the new host_ready, and the
	// info response supplies hello, so discard all earlier signals.
	b.minSignal = infoCall.ResponseSequence
	b.generation++
	generation, file := b.generation, b.notify
	go func() {
		buffer := make([]byte, 65536)
		for {
			n, err := file.Read(buffer)
			p := audioPacket{generation: generation, data: append([]byte(nil), buffer[:n]...), err: err}
			select {
			case b.packets <- p:
			case <-b.ctx.Done():
				return
			}
			if err != nil {
				return
			}
		}
	}()
	b.out.send(bridge.Event{Type: "connected", DeviceID: string(b.selected)})
	b.out.send(bridge.Event{Type: "subscribed"})
	b.out.send(bridge.Event{Type: "control", PayloadBase64: base64.StdEncoding.EncodeToString(hello)})
}
func (b *bluetooth) signal(s *dbus.Signal) {
	if len(s.Body) != 3 || b.selected == "" || s.Sequence <= b.minSignal {
		return
	}
	iface, ok := s.Body[0].(string)
	if !ok {
		return
	}
	p, ok := s.Body[1].(map[string]dbus.Variant)
	if !ok {
		return
	}
	if s.Path == b.selected && iface == deviceIF {
		if connected, ok := p["Connected"].Value().(bool); ok && !connected {
			b.disconnect()
		}
		return
	}
	if s.Path != b.control || iface != charIF {
		return
	}
	data, ok := p["Value"].Value().([]byte)
	if !ok {
		return
	}
	if _, err := protocol.ParseControl(data); err != nil {
		b.fail("蓝牙控制消息无效")
		b.disconnect()
		return
	}
	b.out.send(bridge.Event{Type: "control", PayloadBase64: base64.StdEncoding.EncodeToString(data)})
}
func (b *bluetooth) disconnect() {
	b.generation++
	if b.notify != nil {
		_ = b.notify.Close()
		b.notify = nil
	}
	// Cleanup gets its own short deadline even after stdin closes.
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	if b.control != "" {
		_ = b.bus.Object(bluez, b.control).CallWithContext(ctx, charIF+".StopNotify", 0).Err
	}
	if b.selected != "" {
		_ = b.bus.Object(bluez, b.selected).CallWithContext(ctx, deviceIF+".Disconnect", 0).Err
		b.out.send(bridge.Event{Type: "disconnected"})
	}
	b.selected = ""
	b.control = ""
	b.assembler = protocol.Assembler{}
}
