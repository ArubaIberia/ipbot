package main

import (
	"bytes"
	"fmt"
	"os/exec"
	"strconv"
	"strings"

	"gopkg.in/telegram-bot-api.v4"
)

type VLAN struct {
	Selected   int
	Interfaces *Interfaces
	Device     string
}

// Select a particular VLAN
func (v *VLAN) ReplyToVLAN(msg *tgbotapi.Message, fields []string) string {
	if len(fields) < 2 {
		return "Error: must provide the VLAN number (vlan <vlan_number>)"
	}
	vlan, err := strconv.Atoi(fields[1])
	if err != nil {
		return err.Error()
	}
	if vlan < 1 || vlan > 4094 {
		return "Error: VLAN number must be between 1 and 4094"
	}
	suffix := fmt.Sprintf(".%d", vlan)
	found := ""
	for name := range v.Interfaces.Current {
		if strings.HasSuffix(name, suffix) {
			found = name
			break
		}
	}
	if found == "" {
		return fmt.Sprintf("Error: VLAN %d is not found. Run \"ip\" for more info", vlan)
	}
	v.Selected = vlan
	v.Device = found
	return fmt.Sprintf("VLAN %d selected", vlan)
}

// Add delay in the outbound direction
func (v *VLAN) ReplyToOut(msg *tgbotapi.Message, fields []string) string {
	if v.Selected == 0 {
		return "No VLAN selected. Run \"vlan\" for more info"
	}
	if len(fields) < 3 {
		return "Error: must provide delay and jitter (ms) (out <delay_ms> <jitter_ms>)"
	}
	msDelay, err := strconv.Atoi(fields[1])
	if err != nil {
		return err.Error()
	}
	if msDelay < 1 || msDelay > 4094 {
		return "Error: Delay must be between 1 and 4094 milliseconds"
	}
	msJitter, err := strconv.Atoi(fields[2])
	if err != nil {
		return err.Error()
	}
	if msJitter < 1 || msJitter > 4094 {
		return "Error: Delay must be between 1 and 4094 milliseconds"
	}
	// Remove any qdisc
	cmd := exec.Command("tc", "qdisc", "del", "dev", v.Device, "root")
	cmd.Stdin = strings.NewReader("some input")
	var outDel bytes.Buffer
	cmd.Stdout = &outDel
	if err := cmd.Run(); err != nil {
		return err.Error()
	}
	// Add a new qdisc
	delay := fmt.Sprintf("%dms", msDelay)
	jitter := fmt.Sprintf("%dms", msJitter)
	cmd = exec.Command("tc", "qdisc", "add", "dev", v.Device, "root", "netem", "delay", delay, jitter, "distribution", "normal")
	cmd.Stdin = strings.NewReader("some input")
	var outAdd bytes.Buffer
	cmd.Stdout = &outAdd
	if err := cmd.Run(); err != nil {
		return err.Error()
	}
	// Return the output of the qdisc commands
	return strings.Join([]string{
		outDel.String(),
		outAdd.String(),
	}, "\n")
}
