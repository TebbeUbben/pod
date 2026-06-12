package pod

import (
	"io/ioutil"
	"time"

	toml "github.com/pelletier/go-toml"
	log "github.com/sirupsen/logrus"

	"github.com/avereha/pod/pkg/response"
)

type PODState struct {
	LTK       []byte `toml:"ltk"`
	EapAkaSeq uint64 `toml:"eap_aka_seq"`

	Id []byte `toml:"id"` // 4 byte

	MsgSeq   uint8  `toml:"msg_seq"`   // TODO: is this the same as nonceSeq?
	CmdSeq   uint8  `toml:"cmd_seq"`   // TODO: are all those 3 the same number ???
	NonceSeq uint64 `toml:"nonce_seq"` // or 16?

	LastProgSeqNum uint8 `toml:"last_prog_seq"`

	NoncePrefix []byte `toml:"nonce_prefix"`
	CK          []byte `toml:"ck"`

	PodProgress    response.PodProgress
	ActivationTime time.Time `toml:"activation_time"`

	Reservoir        uint16 `toml:"reservoir"`
	ActiveAlertSlots uint8  `toml:"alerts"`
	FaultEvent       uint8  `toml:"fault"`
	FaultTime        uint16 `toml:"fault_time"`
	Delivered        uint16 `toml:"delivered"`

	TriggerTimes     [8]uint16 `toml:"trigger_times"`

	// At some point these could be replaced with details
	// of each kind of delivery (volume, start time, schedule, etc)
	BolusStart          time.Time     `toml:"bolus_start"`
	BolusEnd            time.Time     `toml:"bolus_end"`
	BolusPulseInterval  time.Duration `toml:"bolus_pulse_interval"`
	BolusTotalPulses    uint16        `toml:"bolus_total_pulses"`
	BolusCanceledAt     time.Time     `toml:"bolus_canceled_at"`
	TempBasalEnd        time.Time     `toml:"temp_basal_end"`
	ExtendedBolusActive bool          `toml:"extended_bolus_active"`
	BasalActive         bool          `toml:"basal_active"`

	Filename string
}

func NewState(filename string) (*PODState, error) {
	var ret PODState
	ret.Filename = filename
	data, err := ioutil.ReadFile(filename)
	if err != nil {
		return nil, err
	}
	err = toml.Unmarshal(data, &ret)
	if err != nil {
		return nil, err
	}
	return &ret, nil
}

func (p *PODState) Save() error {
	log.Debugf("Saving state to file: %s", p.Filename)
	data, err := toml.Marshal(p)
	if err != nil {
		return err
	}
	return ioutil.WriteFile(p.Filename, data, 0777)
}

func (p *PODState) MinutesActive() uint16 {
	return uint16(time.Now().Sub(p.ActivationTime).Round(time.Minute).Minutes())
}

// bolusDeliveredPulses returns the number of bolus pulses delivered so far,
// capped at the total commanded pulses. The delivered count is based on elapsed
// time since bolus start, limited to (BolusEnd - BolusStart).
func (p *PODState) bolusDeliveredPulses() uint16 {
	if p.BolusStart.IsZero() || p.BolusPulseInterval == 0 {
		return 0
	}
	now := time.Now()
	elapsed := now.Sub(p.BolusStart)
	maxElapsed := p.BolusEnd.Sub(p.BolusStart)
	if elapsed > maxElapsed {
		elapsed = maxElapsed
	}
	if elapsed < 0 {
		elapsed = 0
	}
	delivered := uint16(elapsed / p.BolusPulseInterval)
	if delivered > p.BolusTotalPulses {
		delivered = p.BolusTotalPulses
	}
	return delivered
}

// NOTE: only handles immediate boluses; any extended bolus is not accounted for
func (p *PODState) BolusRemaining() uint16 {
	if p.BolusStart.IsZero() || p.BolusTotalPulses == 0 {
		return 0
	}
	delivered := p.bolusDeliveredPulses()
	if delivered >= p.BolusTotalPulses {
		return 0
	}
	return p.BolusTotalPulses - delivered
}

// CurrentDelivered returns the total delivered pulses including any currently
// active bolus pulses delivered so far.
func (p *PODState) CurrentDelivered() uint16 {
	return p.Delivered + p.bolusDeliveredPulses()
}

// CurrentReservoir returns the current reservoir level accounting for any
// pulses delivered by the currently active bolus.
func (p *PODState) CurrentReservoir() uint16 {
	return p.Reservoir - p.bolusDeliveredPulses()
}
