//go:build windows

package main

import (
	"fmt"
	"runtime"
	"slices"
	"syscall"
	"unsafe"
)

// Windows keeps the microphones and the outputs in Core Audio, whose
// interfaces this file calls through their vtables: no cgo, as the rest of
// MyGo. Every call runs on a thread of its own, locked, in COM's
// multithreaded apartment, so that the window's thread never waits for a
// driver.

type guid struct {
	data1 uint32
	data2 uint16
	data3 uint16
	data4 [8]byte
}

// propertyKey is PROPERTYKEY: a format ID and an index into it.
type propertyKey struct {
	fmtid guid
	pid   uint32
}

// propVariant is PROPVARIANT, which is 24 bytes on 64-bit Windows.
type propVariant struct {
	vt         uint16
	wReserved1 uint16
	wReserved2 uint16
	wReserved3 uint16
	val        [2]uintptr
}

var (
	clsidMMDeviceEnumerator = guid{0xbcde0395, 0xe52f, 0x467c, [8]byte{0x8e, 0x3d, 0xc4, 0x57, 0x92, 0x91, 0x69, 0x2e}}
	iidIMMDeviceEnumerator  = guid{0xa95664d2, 0x9614, 0x4f35, [8]byte{0xa7, 0x46, 0xde, 0x8d, 0xb6, 0x36, 0x17, 0xe6}}
	iidIAudioEndpointVolume = guid{0x5cdf2c82, 0x841e, 0x4546, [8]byte{0x97, 0x22, 0x0c, 0xf7, 0x40, 0x78, 0x22, 0x9a}}

	// IPolicyConfig is the interface Windows hides behind its own sound
	// settings: the only way to make an output the default one. It is
	// undocumented, and unchanged since Windows 7.
	clsidPolicyConfig = guid{0x870af99c, 0x171d, 0x4f9e, [8]byte{0xaf, 0x0d, 0xe6, 0x3d, 0xf4, 0x0c, 0x2b, 0xc9}}
	iidPolicyConfig   = guid{0xf8679f50, 0x850a, 0x41cf, [8]byte{0x9c, 0x72, 0x43, 0x0f, 0x29, 0x02, 0x90, 0xc8}}

	// pkeyDeviceFriendlyName is PKEY_Device_FriendlyName, the name Windows
	// shows the user for a device.
	pkeyDeviceFriendlyName = propertyKey{guid{0xa45c254e, 0xdf1c, 0x4efd, [8]byte{0x80, 0x20, 0x67, 0xd1, 0x46, 0xa8, 0x50, 0xe0}}, 14}

	ole32                = syscall.NewLazyDLL("ole32.dll")
	procCoInitializeEx   = ole32.NewProc("CoInitializeEx")
	procCoCreateInstance = ole32.NewProc("CoCreateInstance")
	procCoTaskMemFree    = ole32.NewProc("CoTaskMemFree")
	procPropVariantClear = ole32.NewProc("PropVariantClear")
)

const (
	clsctxAll           = 0x17
	coinitMultithreaded = 0
	eRender             = 0
	eCapture            = 1
	eConsole            = 0
	eMultimedia         = 1
	eCommunications     = 2
	deviceStateActive   = 0x1
	vtLPWStr            = 31
	hardwareSupportMute = 0x2
	stgmRead            = 0

	// iPolicyConfigSetDefault is where IPolicyConfig puts
	// SetDefaultEndpoint: after the formats, the periods, the share mode
	// and the properties.
	iPolicyConfigSetDefault = 13
)

// comCall calls method i of the COM interface obj. Pointers converted to
// uintptr in the call stay valid until it returns (go:uintptrescapes).
//
//go:uintptrescapes
func comCall(obj uintptr, i int, args ...uintptr) uintptr {
	vtbl := *(*uintptr)(native(obj))
	fn := *(*uintptr)(unsafe.Add(native(vtbl), i*int(unsafe.Sizeof(uintptr(0)))))
	var a [16]uintptr
	a[0] = obj
	n := copy(a[1:], args)
	r, _, _ := syscall.SyscallN(fn, a[:n+1]...)
	return r
}

// native converts a native address to a pointer.
func native(addr uintptr) unsafe.Pointer { return *(*unsafe.Pointer)(unsafe.Pointer(&addr)) }

func failed(hr uintptr) bool { return int32(uint32(hr)) < 0 }

func hresult(what string, hr uintptr) error {
	return fmt.Errorf("%s failed (HRESULT %#08x)", what, uint32(hr))
}

// release drops a COM interface.
func release(obj uintptr) {
	if obj != 0 {
		comCall(obj, 2)
	}
}

func releaseAll(objs []uintptr) {
	for _, o := range objs {
		release(o)
	}
}

// coreAudio runs Core Audio's calls on a thread of its own.
type coreAudio struct {
	calls chan func()
	// reader is what the reads remember between them, which the audio
	// thread alone touches.
	reader audioReader
}

func newAudioController() (audioController, error) {
	a := &coreAudio{calls: make(chan func())}
	ready := make(chan error, 1)
	go a.run(ready)
	if err := <-ready; err != nil {
		return nil, err
	}
	return a, nil
}

// run owns the thread that every COM call of this tool runs on.
func (a *coreAudio) run(ready chan<- error) {
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	// S_FALSE, already initialized, succeeds as well.
	if hr, _, _ := procCoInitializeEx.Call(0, coinitMultithreaded); failed(hr) {
		ready <- hresult("CoInitializeEx", hr)
		return
	}
	ready <- nil
	for fn := range a.calls {
		fn()
	}
}

// do runs fn on the audio thread and waits for it.
func (a *coreAudio) do(fn func()) {
	done := make(chan struct{})
	a.calls <- func() {
		defer close(done)
		fn()
	}
	<-done
}

func (a *coreAudio) State() (audioState, error) {
	var st audioState
	var err error
	a.do(func() { st, err = a.reader.read() })
	return st, err
}

func (a *coreAudio) SetMuted(muted bool) error {
	var err error
	a.do(func() { err = switchMuted(muted) })
	return err
}

func (a *coreAudio) Outputs() ([]outputDevice, error) {
	var out []outputDevice
	var err error
	a.do(func() { out, err = listOutputs() })
	return out, err
}

func (a *coreAudio) SetOutput(id string) error {
	var err error
	a.do(func() { err = setDefaultOutput(id) })
	return err
}

// newEnumerator opens Core Audio's list of devices.
func newEnumerator() (uintptr, error) {
	var enum uintptr
	if hr, _, _ := procCoCreateInstance.Call(uintptr(unsafe.Pointer(&clsidMMDeviceEnumerator)), 0, clsctxAll,
		uintptr(unsafe.Pointer(&iidIMMDeviceEnumerator)), uintptr(unsafe.Pointer(&enum))); failed(hr) {
		return 0, hresult("CoCreateInstance(MMDeviceEnumerator)", hr)
	}
	return enum, nil
}

// endpoints returns the devices of a flow the computer has switched on, the
// microphones or the outputs. The caller releases them.
func endpoints(enum uintptr, flow uintptr) ([]uintptr, error) {
	var coll uintptr
	if hr := comCall(enum, 3, flow, deviceStateActive, uintptr(unsafe.Pointer(&coll))); failed(hr) {
		return nil, hresult("IMMDeviceEnumerator.EnumAudioEndpoints", hr)
	}
	defer release(coll)
	var n uint32
	if hr := comCall(coll, 3, uintptr(unsafe.Pointer(&n))); failed(hr) {
		return nil, hresult("IMMDeviceCollection.GetCount", hr)
	}
	devices := make([]uintptr, 0, n)
	for i := uint32(0); i < n; i++ {
		var dev uintptr
		if hr := comCall(coll, 4, uintptr(i), uintptr(unsafe.Pointer(&dev))); failed(hr) {
			continue
		}
		devices = append(devices, dev)
	}
	return devices, nil
}

// defaultDevice returns the device of a flow Windows gives apps, the first
// of roles it has. The caller releases it.
func defaultDevice(enum uintptr, flow uintptr, roles ...uintptr) (uintptr, error) {
	var last error
	for _, role := range roles {
		var dev uintptr
		if hr := comCall(enum, 4, flow, role, uintptr(unsafe.Pointer(&dev))); failed(hr) {
			last = hresult("GetDefaultAudioEndpoint", hr)
			continue
		}
		return dev, nil
	}
	return 0, last
}

// deviceID reads the endpoint ID of a device.
func deviceID(dev uintptr) string {
	var p *uint16
	if hr := comCall(dev, 5, uintptr(unsafe.Pointer(&p))); failed(hr) || p == nil {
		return ""
	}
	defer procCoTaskMemFree.Call(uintptr(unsafe.Pointer(p)))
	n := 0
	for *(*uint16)(unsafe.Add(unsafe.Pointer(p), n*2)) != 0 {
		n++
	}
	return syscall.UTF16ToString(unsafe.Slice(p, n))
}

// friendlyName reads PKEY_Device_FriendlyName of a device.
func friendlyName(dev uintptr) string {
	var store uintptr
	if hr := comCall(dev, 4, stgmRead, uintptr(unsafe.Pointer(&store))); failed(hr) {
		return ""
	}
	defer release(store)
	var pv propVariant
	if hr := comCall(store, 5, uintptr(unsafe.Pointer(&pkeyDeviceFriendlyName)), uintptr(unsafe.Pointer(&pv))); failed(hr) {
		return ""
	}
	defer procPropVariantClear.Call(uintptr(unsafe.Pointer(&pv)))
	if pv.vt != vtLPWStr || pv.val[0] == 0 {
		return ""
	}
	p := (*uint16)(native(pv.val[0]))
	n := 0
	for *(*uint16)(unsafe.Add(unsafe.Pointer(p), n*2)) != 0 {
		n++
	}
	return syscall.UTF16ToString(unsafe.Slice(p, n))
}

// slowEvery is how many reads go by between the ones that ask the computer
// what rarely changes: what the devices are called, and how many outputs it
// has. The nine reads between them take what the last one found, since
// asking costs more than those answers change.
const slowEvery = 10

// maxNames bounds the names a reader keeps, for a computer whose devices
// come and go for long enough to fill it.
const maxNames = 64

// audioReader reads what the microphones and the output do, remembering what
// it read: a name and an endpoint's volume stay as they were for as long as
// the endpoint does, and asking the audio service for them again every
// second is most of what a read otherwise costs.
type audioReader struct {
	// reads counts the reads, for the one in slowEvery that asks what
	// rarely changes.
	reads int
	// names is what each endpoint is called, by the endpoint ID that name
	// never changes under.
	names map[string]string
	// volumes is the volume of each microphone, which holds its mute, kept
	// open rather than opened again by every read: Activate is the most
	// expensive call a read makes.
	volumes map[string]uintptr
	// outputs is how many outputs the computer had when the reader last
	// counted them, which the card needs only to know whether there is
	// more than one to choose among.
	outputs int
}

// read asks the computer what its microphones and its output do.
func (r *audioReader) read() (audioState, error) {
	// Every tenth read asks what rarely changes, and the nine between it
	// take what it found.
	slow := r.reads%slowEvery == 0
	r.reads++

	enum, err := newEnumerator()
	if err != nil {
		return audioState{}, err
	}
	defer release(enum)

	st := audioState{}
	if dev, err := defaultDevice(enum, eCapture, eCommunications, eConsole); err == nil {
		st.Mic.Device = r.name(dev, deviceID(dev), slow)
		release(dev)
	}
	if devices, err := endpoints(enum, eCapture); err == nil {
		st.Mic.Count = len(devices)
		st.Mic.Muted = len(devices) > 0
		ids := make([]string, 0, len(devices))
		for _, dev := range devices {
			id := deviceID(dev)
			ids = append(ids, id)
			// A microphone that cannot say is not reported as muted: the
			// card must never claim silence the computer does not give.
			if muted, err := r.muted(dev, id); err != nil || !muted {
				st.Mic.Muted = false
			}
		}
		r.keep(ids)
		releaseAll(devices)
	}
	if dev, err := defaultDevice(enum, eRender, eConsole, eCommunications); err == nil {
		id := deviceID(dev)
		st.Output = outputDevice{ID: id, Name: r.name(dev, id, slow)}
		release(dev)
	}
	if slow {
		if outputs, err := endpoints(enum, eRender); err == nil {
			r.outputs = len(outputs)
			releaseAll(outputs)
		}
	}
	st.Outputs = r.outputs
	return st, nil
}

// name returns what a device is called, from what the reader kept unless
// fresh asks the computer again.
func (r *audioReader) name(dev uintptr, id string, fresh bool) string {
	if !fresh && id != "" {
		if name, ok := r.names[id]; ok {
			return name
		}
	}
	name := friendlyName(dev)
	if id == "" || name == "" {
		return name
	}
	if r.names == nil {
		r.names = make(map[string]string)
	}
	if len(r.names) >= maxNames {
		clear(r.names)
	}
	r.names[id] = name
	return name
}

// muted returns whether a microphone is muted, from the volume the reader
// keeps open for it.
func (r *audioReader) muted(dev uintptr, id string) (bool, error) {
	vol, err := r.volume(dev, id)
	if err != nil {
		return false, err
	}
	muted, hr := volumeMute(vol)
	if failed(hr) && id != "" {
		// The endpoint the volume was opened on may be gone: open it again,
		// once, for the device this read found.
		r.forget(id)
		if vol, err = r.volume(dev, id); err != nil {
			return false, err
		}
		if muted, hr = volumeMute(vol); failed(hr) {
			r.forget(id)
		}
	}
	if id == "" {
		// A device that cannot say its ID keeps no volume open: it has one
		// for this read alone.
		release(vol)
	}
	if failed(hr) {
		return false, hresult("IAudioEndpointVolume.GetMute", hr)
	}
	return muted, nil
}

// volumeMute asks a volume whether it is muted.
func volumeMute(vol uintptr) (bool, uintptr) {
	var muted int32
	hr := comCall(vol, 15, uintptr(unsafe.Pointer(&muted)))
	return muted != 0, hr
}

// volume returns the volume of a device, which holds its mute too, opened
// once and kept.
func (r *audioReader) volume(dev uintptr, id string) (uintptr, error) {
	if id != "" {
		if vol, ok := r.volumes[id]; ok {
			return vol, nil
		}
	}
	vol, err := endpointVolume(dev)
	if err != nil {
		return 0, err
	}
	if id != "" {
		if r.volumes == nil {
			r.volumes = make(map[string]uintptr)
		}
		r.volumes[id] = vol
	}
	return vol, nil
}

// forget drops the volume the reader kept of an endpoint whose interface no
// longer works, so that the next read opens it again.
func (r *audioReader) forget(id string) {
	if vol, ok := r.volumes[id]; ok {
		release(vol)
		delete(r.volumes, id)
	}
}

// keep drops the volumes of endpoints the read did not find, which a
// microphone that came unplugged leaves behind.
func (r *audioReader) keep(ids []string) {
	for id, vol := range r.volumes {
		if !slices.Contains(ids, id) {
			release(vol)
			delete(r.volumes, id)
		}
	}
}

// endpointVolume returns the volume of a device, which holds its mute too.
func endpointVolume(dev uintptr) (uintptr, error) {
	var vol uintptr
	if hr := comCall(dev, 3, uintptr(unsafe.Pointer(&iidIAudioEndpointVolume)), clsctxAll, 0,
		uintptr(unsafe.Pointer(&vol))); failed(hr) {
		return 0, hresult("IMMDevice.Activate", hr)
	}
	return vol, nil
}

// switchMuted mutes or unmutes every microphone of the computer: an app
// that opens one of them, the default or another, hears silence.
func switchMuted(muted bool) error {
	enum, err := newEnumerator()
	if err != nil {
		return err
	}
	defer release(enum)

	devices, err := endpoints(enum, eCapture)
	if err != nil {
		return err
	}
	defer releaseAll(devices)
	if len(devices) == 0 {
		return errNoMicrophone
	}

	var first error
	switched := 0
	for _, dev := range devices {
		if err := muteDevice(dev, muted); err != nil {
			if first == nil {
				first = err
			}
			continue
		}
		switched++
	}
	if switched == 0 && first != nil {
		return first
	}
	return nil
}

func muteDevice(dev uintptr, muted bool) error {
	vol, err := endpointVolume(dev)
	if err != nil {
		return err
	}
	defer release(vol)
	var support uint32
	if hr := comCall(vol, 19, uintptr(unsafe.Pointer(&support))); !failed(hr) && support&hardwareSupportMute == 0 {
		return errNoMuteSupport
	}
	var b uintptr
	if muted {
		b = 1
	}
	// SetMute(bMute, pguidEventContext): no event context, so that the
	// system's own listeners do not take the change for one of theirs.
	if hr := comCall(vol, 14, b, 0); failed(hr) {
		return hresult("IAudioEndpointVolume.SetMute", hr)
	}
	return nil
}

// listOutputs lists the outputs the computer has, the default one among
// them.
func listOutputs() ([]outputDevice, error) {
	enum, err := newEnumerator()
	if err != nil {
		return nil, err
	}
	defer release(enum)

	devices, err := endpoints(enum, eRender)
	if err != nil {
		return nil, err
	}
	defer releaseAll(devices)
	if len(devices) == 0 {
		return nil, errNoOutput
	}

	current := ""
	if dev, err := defaultDevice(enum, eRender, eConsole, eCommunications); err == nil {
		current = deviceID(dev)
		release(dev)
	}
	outputs := make([]outputDevice, 0, len(devices))
	for _, dev := range devices {
		id := deviceID(dev)
		outputs = append(outputs, outputDevice{ID: id, Name: friendlyName(dev), Default: id == current})
	}
	return outputs, nil
}

// setDefaultOutput makes an output the device apps play to. Windows keeps
// three defaults, for media, for calls and for the rest: a device is only
// the computer's output when it is all three.
func setDefaultOutput(id string) error {
	var policy uintptr
	if hr, _, _ := procCoCreateInstance.Call(uintptr(unsafe.Pointer(&clsidPolicyConfig)), 0, clsctxAll,
		uintptr(unsafe.Pointer(&iidPolicyConfig)), uintptr(unsafe.Pointer(&policy))); failed(hr) {
		return hresult("CoCreateInstance(PolicyConfig)", hr)
	}
	defer release(policy)
	name, err := syscall.UTF16PtrFromString(id)
	if err != nil {
		return err
	}
	for _, role := range []uintptr{eConsole, eMultimedia, eCommunications} {
		if hr := comCall(policy, iPolicyConfigSetDefault, uintptr(unsafe.Pointer(name)), role); failed(hr) {
			return hresult("IPolicyConfig.SetDefaultEndpoint", hr)
		}
	}
	return nil
}
