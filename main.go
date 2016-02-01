package main

import (
	"fmt"
	"log"
	"time"

	"github.com/dasfoo/bright-pi"
	"github.com/dasfoo/i2c"
	"github.com/dasfoo/lidar-lite-v2"
	"github.com/dasfoo/rover/bb"
)

func ledCircle(b *bpi.BrightPI, circles int) {
	leds := []byte{bpi.WhiteTopLeft, bpi.WhiteTopRight, bpi.WhiteBottomRight, bpi.WhiteBottomLeft}
	for i := 0; i < circles; i++ {
		for _, led := range leds {
			_ = b.Power(led)
			time.Sleep(50 * time.Millisecond)
		}
	}
}

func main() {
	log.SetFlags(log.LstdFlags | log.Lshortfile | log.Lmicroseconds)
	bus, err := i2c.NewBus(1)
	if err != nil {
		log.Println(err)
		return
	}

	board := bb.NewBB(bus, bb.Address)
	_ = board.Wake()

	if s, e := board.GetStatus(); e == nil {
		fmt.Printf("Status bits: %.16b\n", s)
	}

	if p, e := board.GetBatteryPercentage(); e != nil {
		board.Reset(bb.ResetPin)
		time.Sleep(time.Second)
	} else {
		fmt.Println("Battery status (estimated):", p, "%")
	}

	if l, e := board.GetBrightness(); e == nil {
		fmt.Println("Brightness (0-1023):", l)
	}

	/*board.MotorRight(bb.MaxMotorSpeed / 2)
	time.Sleep(100 * time.Millisecond)
	board.MotorRight(0)
	time.Sleep(time.Second)
	board.MotorLeft(bb.MaxMotorSpeed / 2)
	time.Sleep(100 * time.Millisecond)
	board.MotorLeft(0)*/

	_ = board.Pan(30)
	_ = board.Tilt(45)
	time.Sleep(time.Second)
	_ = board.Pan(90)
	_ = board.Tilt(0)

	if t, h, e := board.GetTemperatureAndHumidity(); e == nil {
		fmt.Println("Temperature", t, "*C, humidity", h, "%")
	}

	b := bpi.NewBrightPI(bus, bpi.DefaultAddress)
	s := lidar.NewLidar(bus, lidar.DefaultAddress)
	_ = s.Reset()

	_ = b.Dim(bpi.WhiteAll, bpi.DefaultDim)
	_ = b.Gain(bpi.DefaultGain)
	go ledCircle(b, 2)

	// Simple GetDistance
	fmt.Println(s.GetDistance())

	// Continuous mode
	/*_ = s.SetContinuousMode(50, 100*time.Millisecond)
	_ = s.Acquire(false)
	for i := 0; i < 50; i++ {
		fmt.Println("Round", i)
		fmt.Println(s.ReadDistance())
		fmt.Println(s.ReadVelocity())
		time.Sleep(100 * time.Millisecond)
	}*/

	if hw, sw, err := s.GetVersion(); err == nil {
		fmt.Printf("LIDAR-Lite v2 \"Blue Label\" hw%dsw%d\n", hw, sw)
	}

	time.Sleep(time.Second)

	// Put devices in low power consumption mode
	_ = s.Sleep()
	_ = b.Sleep()
	_ = board.Sleep()
}
