package main

import "errors"

// Errors the card names in words of its own.
var (
	// errNoMicrophone is a computer with no microphone to switch.
	errNoMicrophone = errors.New("no microphone")
	// errNoOutput is a computer with no output to switch.
	errNoOutput = errors.New("no output")
	// errNoMuteSupport is a microphone whose driver cannot mute it.
	errNoMuteSupport = errors.New("the microphone cannot be muted")
	// errUnsupported is a system this tool does not drive yet.
	errUnsupported = errors.New("unsupported system")
)

// micState is what the microphones of the computer do now.
type micState struct {
	// Device names the microphone the computer gives apps by default, or
	// "" when it has none.
	Device string
	// Muted reports whether every microphone of the computer is muted.
	Muted bool
	// Count is how many microphones the computer has.
	Count int
}

// outputDevice is a place the computer plays sound to.
type outputDevice struct {
	// ID is the endpoint ID Windows knows the device by, which switching
	// the default takes.
	ID string
	// Name is the friendly name Windows shows the user for it.
	Name string
	// Default reports whether apps play to it now, as a list of outputs
	// finds them.
	Default bool
}

// audioState is what the card shows: the microphones and the output.
type audioState struct {
	Mic micState
	// Output is the device the computer plays to by default, with an empty
	// ID when it has none.
	Output outputDevice
	// Outputs is how many outputs the computer has.
	Outputs int
}

// audioController reads and switches the microphones and the outputs of the
// computer. Every method is safe to call from any goroutine and does not
// block for long.
type audioController interface {
	State() (audioState, error)
	SetMuted(muted bool) error
	// Outputs lists the outputs the computer has, the default one among
	// them.
	Outputs() ([]outputDevice, error)
	// SetOutput makes an output the device apps play to, for media and for
	// calls alike.
	SetOutput(id string) error
}

// brokenAudio is audio this computer cannot switch: it fails every call
// with err, so that the card says why instead of doing nothing.
type brokenAudio struct{ err error }

func (b brokenAudio) State() (audioState, error)       { return audioState{}, b.err }
func (b brokenAudio) SetMuted(bool) error              { return b.err }
func (b brokenAudio) Outputs() ([]outputDevice, error) { return nil, b.err }
func (b brokenAudio) SetOutput(string) error           { return b.err }
