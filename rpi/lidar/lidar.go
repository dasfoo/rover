package lidar

import (
	"errors"
	"log"
	"time"

	"github.com/kidoman/embd"
	_ "github.com/kidoman/embd/host/all" // for multi-host support
)

// Lidar is structure to access basic functions of LidarLite
// LidarLite_V2 blue label
// Model LL-905-PIN-02
// Documentation on http://lidarlite.com/docs/v2/specs_and_hardware
type Lidar struct {
	bus     embd.I2CBus
	address byte
}

// MaxAttemptNumber - maximum number of attempts to do operation
const MaxAttemptNumber = 50

const (
	// NotReady - Ready status. 0 - ready for a new command, 1 - busy
	// with acquisition.
	NotReady = 1 << iota

	// RefOverflow - Overflow detected in correlation process assotieted with
	// a reference acquisition. Signal overflow flag and Reference overflow flag
	// are set when automatic limiting occurs.
	RefOverflow = 1 << iota

	// SignalOverflow - Overflow detected in correlation process assotieted with a signal
	// acquisition
	SignalOverflow = 1 << iota

	// SignalNotValid - Indicates that the signal correlation peak is equal to or below
	// correlation record threshold
	SignalNotValid = 1 << iota

	// SecondaryReturn - Secondary return detected above correlation noise floor threshold
	SecondaryReturn = 1 << iota

	// Health - 1 if is good, 0 if is bad
	Health = 1 << iota

	// ErrorDetection - Process error detected/measurement invalid
	ErrorDetection = 1 << iota

	// EyeSafe - This bit will go high if eye-safety protection has been activated
	EyeSafe = 1 << iota
)

// NewLidar sets the configuration for the sensor
// Write 0x00 to Register 0x00 reset FPGA. Re-loads FPGA from internal Flash
// memory: all registers return to default values
// During initialization the microcontroller goes throw a self-test followed by
// initialization of the internal control registers with default values. After
// processor goes into sleep state reducing overall power consumption to under
// 10 mA. Initiation of a user command, throw external trigger or I2C command,
// awakes a processor allowing subsequent opetation.
func NewLidar(i2cbus, addr byte) *Lidar {
	lSensor := Lidar{bus: embd.NewI2CBus(i2cbus), address: addr}
	if e := lSensor.bus.WriteByteToReg(lSensor.address, 0x00, 0x00); e != nil {
		log.Panic("Write ", e)
	}
	log.Println("Initialization is done")
	time.Sleep(1 * time.Second)
	return &lSensor
}

// Read reads from the register and the same time check status of controller.
// If Status is bad, it tries again
func (ls *Lidar) Read(register byte) (byte, error) {
	for i := 0; i < MaxAttemptNumber; i++ {
		st, errSt := ls.GetStatus()
		switch {
		case errSt != nil:
			log.Println(errSt)
		case st&Health == 0:
			log.Println("Bad Health of controller")
			val, rErr := ls.bus.ReadByteFromReg(ls.address, register)
			if rErr == nil {
				return val, nil
			}
		case st&ErrorDetection > 0:
			log.Println("Error detected")
		case st&SignalOverflow == 0:
			log.Println("Automatic limiting doesn't occurs ")
		default:
			val, rErr := ls.bus.ReadByteFromReg(ls.address, register)
			if rErr == nil {
				return val, nil
			}
		}
		//if ask Status often, Health status is bad
		time.Sleep(1 * time.Second)
	}
	return 0, errors.New("Read limit occurs")
}

// WriteByteToRegister - write value(byte) to register(reg)
// Read register 0x01(this is handled in the GetStatus() command)
//  - if the first bit is "1"(it checks in NotReady) then sensor is busy, loop
//    until the first bit is 0 or i = MaxAttemptNumber
//  - if the first bit is "0"(it checks in NotReady) then the sensor is ready
//    for a new command
func (ls *Lidar) WriteByteToRegister(register, value byte) error {

	for i := 0; i < MaxAttemptNumber; i++ {
		st, errSt := ls.GetStatus()
		switch {
		case errSt != nil:
			log.Println(errSt)
		case st&NotReady > 0:
			log.Println("Not ready to start new command")
		default:
			return ls.bus.WriteByteToReg(ls.address, register, value)
		}
		time.Sleep(1 * time.Second)
	}
	return errors.New("Write limit occurs")
}

//Close closes releases the resources associated with the bus
func (ls *Lidar) Close() {
	// Reset FPGA. All registers return to default values
	if e := ls.bus.WriteByteToReg(ls.address, 0x00, 0x00); e != nil {
		log.Println("Write ", e)
	}
	if err := ls.bus.Close(); err != nil {
		log.Println(err)
	}
	log.Println("Closing sensor is done")
}

// GetStatus gets Mode/Status of sensor
func (ls *Lidar) GetStatus() (byte, error) {

	val, err := ls.bus.ReadByteFromReg(ls.address, 0x01)
	if err != nil {
		log.Println("GetStatus", err)
		return 0, err
	}
	log.Printf("Status: %.8b\n", val)
	return val, nil
}

// Distance reads the distance from LidarLite
// stablizePreampFlag - true - take aquisition with DC stabilisation/correction.
// false - it will read faster, but you will need to stabilize DC every once in
// awhile(ex. 1 out of every 100 readings is typically good)
func (ls *Lidar) Distance(stablizePreampFlag bool) (int, error) {

	var wErr error // Write error

	if stablizePreampFlag {
		wErr = ls.WriteByteToRegister(0x00, 0x04)
	} else {
		wErr = ls.WriteByteToRegister(0x00, 0x03)
	}
	if wErr != nil {
		log.Println("Write ", wErr)
		return -1, wErr
	}

	// The total acquisition time for the reference and signal acquisition is
	// typically between 5 and 20 ms depending on the desired number of integrated
	// pulses and the length of the correlation record. The acquisition time
	// plus the required 1 msec to download measurement parameters establish a
	// a roughly 100Hz maximum measurement rate.
	time.Sleep(250 * time.Millisecond)

	v1, rErr := ls.Read(0x10)
	if rErr != nil {
		log.Println("Read ", rErr)
		return -1, rErr
	}
	v2, rErr := ls.Read(0x0f)
	if rErr != nil {
		log.Println("Read", rErr)
		return -1, rErr
	}

	return ((int(v2) << 8) + int(v1)), nil

}

// Velocity is measured by observing the change in distance over a fixed time
// of period
// TODO 0x04 Check Mode Control
// TODO Check unit
func (ls *Lidar) Velocity() (int, error) {
	// Write 0xa0 to 0x04 to switch on velocity mode
	// Before changing mode we need to check status
	log.Println("Starting velocity mode")
	if wErr := ls.WriteByteToRegister(0x04, 0xa0); wErr != nil {
		log.Println("Write ", wErr)
		return -1, wErr
	}

	// Write 0x04 to register 0x00 to start getting distance readings
	if wErr := ls.bus.WriteByteToReg(ls.address, 0x00, 0x04); wErr != nil {
		log.Println("Write ", wErr)
		return -1, wErr
	}
	log.Println("Velocity  reading....")

	//Read 1 byte from register 0x09 to get velocity measurement
	val, e := ls.Read(0x09)
	if e != nil {
		log.Println(e)
		return -1, e
	}
	return int(val), nil

}

// BeginContinuous allows to tell the sensor to take a certain number (or
// infinite) readings allowing you to read from it at a continuous rate.
// - modePinLow - tells the mode pin to go low when a new reading is available.
// - interval - set the time between measurements, default is 0x04.
//   0xc8 corresponds to 10Hz while 0x13 corresponds to 100Hz. Maximum
//   value is 0x02 for proper operations
// - numberOfReadings - set the number of readings to take before stopping
func (ls *Lidar) BeginContinuous(modePinLow bool, interval, numberOfReadings byte) error {

	// Register 0x45 sets the time between measurements. Min val os 0x02
	// for proper operations.
	if wErr := ls.bus.WriteByteToReg(ls.address, 0x45, interval); wErr != nil {
		log.Println(wErr)
		return wErr
	}

	if modePinLow {
		if wErr := ls.bus.WriteByteToReg(ls.address, 0x04, 0x21); wErr != nil {
			log.Print(wErr)
			return wErr
		}

	} else {
		// Set register 0x04 to 0x20 to look at "NON-default" value of velocity
		if wErr := ls.bus.WriteByteToReg(ls.address, 0x04, 0x20); wErr != nil {
			log.Print(wErr)
			return wErr
		}
	}
	// Set the number of readings, 0xfe = 254 readings, 0x01 = 1 reading and
	// 0xff = continuous readings
	if wErr := ls.bus.WriteByteToReg(ls.address, 0x11, numberOfReadings); wErr != nil {
		log.Println(wErr)
		return wErr
	}

	// Initiate reading distance
	if wErr := ls.bus.WriteByteToReg(ls.address, 0x00, 0x04); wErr != nil {
		log.Println(wErr)
		return wErr
	}
	time.Sleep(1 * time.Second)
	return nil
}

// DistanceContinuous reads in continuous mode
// TODO Status check
func (ls *Lidar) DistanceContinuous() (int, error) {

	status, err := ls.GetStatus()
	switch {
	case err != nil:
		log.Println(err)
		return -1, err
	case status&Health == 0:
		val, rErr := ls.bus.ReadWordFromReg(ls.address, 0x8f)
		log.Println("Bad health of sensor")
		if rErr != nil {
			log.Println("Read", rErr)
			return -1, rErr
		}
		return int(val), nil
	case status&ErrorDetection > 0:
		return -1, errors.New("Error in counting detected")
	case status&SignalOverflow == 0:
		return -1, errors.New("Automatic limiting doesn't occurs")
	default:
		val, rErr := ls.bus.ReadWordFromReg(ls.address, 0x8f)
		if rErr != nil {
			log.Println("Read", rErr)
			return -1, rErr
		}
		return int(val), nil
	}
}
