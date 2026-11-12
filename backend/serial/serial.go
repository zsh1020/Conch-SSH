// Package serial provides USB/serial port terminal sessions.
package serial

import (
	"fmt"
	"strings"
	"sync"

	"go.bug.st/serial"
	"go.bug.st/serial/enumerator"
)

// Options holds serial port connection parameters.
type Options struct {
	Port     string // e.g. "COM3" / "/dev/ttyUSB0"
	BaudRate int    // e.g. 115200
	DataBits int    // 5/6/7/8, default 8
	Parity   string // "none" / "odd" / "even", default none
	StopBits string // "1" / "1.5" / "2", default "1"
}

// Session is one open serial port with a background read pump.
type Session struct {
	port      serial.Port
	closeOnce sync.Once
}

// PortInfo describes one serial port: its device name and, when the OS
// provides one, a friendly description (e.g. the USB product string).
type PortInfo struct {
	Name        string `json:"name"`        // "COM3" / "/dev/ttyUSB0"
	Description string `json:"description"` // "Silicon Labs CP210x USB to UART Bridge", may be empty
}

// ListPorts returns the available serial ports with friendly descriptions.
// Falls back to bare names when detailed enumeration is unavailable.
func ListPorts() ([]PortInfo, error) {
	details, err := enumerator.GetDetailedPortsList()
	if err != nil {
		names, err2 := serial.GetPortsList()
		if err2 != nil {
			return nil, fmt.Errorf("list serial ports: %w", err2)
		}
		infos := make([]PortInfo, 0, len(names))
		for _, n := range names {
			infos = append(infos, PortInfo{Name: n})
		}
		return infos, nil
	}
	infos := make([]PortInfo, 0, len(details))
	for _, d := range details {
		// 友好名通常自带 " (COMx)" 后缀，去掉以免重复显示
		desc := strings.TrimSuffix(d.Product, " ("+d.Name+")")
		infos = append(infos, PortInfo{Name: d.Name, Description: desc})
	}
	return infos, nil
}

func parityOf(s string) serial.Parity {
	switch s {
	case "odd":
		return serial.OddParity
	case "even":
		return serial.EvenParity
	default:
		return serial.NoParity
	}
}

func stopBitsOf(s string) serial.StopBits {
	switch s {
	case "1.5":
		return serial.OnePointFiveStopBits
	case "2":
		return serial.TwoStopBits
	default:
		return serial.OneStopBit
	}
}

// Open opens the port and starts pumping incoming bytes to onOutput.
// onClose is called exactly once when the port goes away (read error,
// device unplugged) or Close is invoked.
func Open(opts Options, onOutput func([]byte), onClose func()) (*Session, error) {
	dataBits := opts.DataBits
	if dataBits < 5 || dataBits > 8 {
		dataBits = 8
	}
	mode := &serial.Mode{
		BaudRate: opts.BaudRate,
		DataBits: dataBits,
		Parity:   parityOf(opts.Parity),
		StopBits: stopBitsOf(opts.StopBits),
	}
	if mode.BaudRate <= 0 {
		mode.BaudRate = 115200
	}

	port, err := serial.Open(opts.Port, mode)
	if err != nil {
		return nil, fmt.Errorf("open %s: %w", opts.Port, err)
	}

	s := &Session{port: port}

	go func() {
		buf := make([]byte, 32*1024)
		for {
			n, err := s.port.Read(buf)
			if n > 0 {
				chunk := make([]byte, n)
				copy(chunk, buf[:n])
				onOutput(chunk)
			}
			if err != nil {
				s.Close()
				onClose()
				return
			}
		}
	}()

	return s, nil
}

// Write sends bytes to the serial port.
func (s *Session) Write(p []byte) (int, error) {
	return s.port.Write(p)
}

// Close releases the serial port. Safe to call multiple times.
func (s *Session) Close() error {
	var err error
	s.closeOnce.Do(func() {
		err = s.port.Close()
	})
	return err
}
