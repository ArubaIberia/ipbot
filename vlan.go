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
func (v *VLAN) ReplyToVLAN(msg *tgbotapi.Message, fields []string) (string, []string) {
	if len(fields) < 2 {
		return "Error: must provide the VLAN number (vlan <vlan_number>)", nil
	}
	vlan, err := strconv.Atoi(fields[1])
	if err != nil {
		return err.Error(), nil
	}
	if vlan < 1 || vlan > 4094 {
		return "Error: VLAN number must be between 1 and 4094", nil
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
		return fmt.Sprintf("Error: VLAN %d is not found. Run \"ip\" for more info", vlan), nil
	}
	v.Selected = vlan
	v.Device = found
	return fmt.Sprintf("VLAN %d selected", vlan), fields[2:]
}

type params struct {
	delay, jitter int
	err           string
}

func (v *VLAN) getParams(msg *tgbotapi.Message, fields []string) (params, []string) {
	if v.Selected == 0 {
		return params{err: "No VLAN selected. Run \"vlan\" for more info"}, nil
	}
	if len(fields) < 3 {
		return params{err: "Error: must provide delay and jitter (ms) (out <delay_ms> <jitter_ms>)"}, nil
	}
	msDelay, err := strconv.Atoi(fields[1])
	if err != nil {
		return params{err: fmt.Sprintf("delay is not an int: %s", err.Error())}, nil
	}
	if msDelay < 1 || msDelay > 4094 {
		return params{err: "Error: Delay must be between 1 and 4094 milliseconds"}, nil
	}
	msJitter, err := strconv.Atoi(fields[2])
	if err != nil {
		return params{err: fmt.Sprintf("jitter is not an int: %s", err.Error())}, nil
	}
	if msJitter < 1 || msJitter > 4094 {
		return params{err: "Error: Delay must be between 1 and 4094 milliseconds"}, nil
	}
	return params{delay: msDelay, jitter: msJitter}, fields[3:]
}

// Add delay to an interface
func (v *VLAN) delay(iface string, msDelay, msJitter int, remainder []string) (string, []string) {
	// Remove any qdisc
	cmd := exec.Command("tc", "qdisc", "del", "dev", iface, "root")
	cmd.Stdin = strings.NewReader("some input")
	var outDel bytes.Buffer
	cmd.Stdout = &outDel
	header := ""
	if err := cmd.Run(); err != nil {
		header = fmt.Sprintf("(Ignore) Error at qdisc del: %s", err.Error())
	}
	// Add a new qdisc
	delay := fmt.Sprintf("%dms", msDelay)
	jitter := fmt.Sprintf("%dms", msJitter)
	cmd = exec.Command("tc", "qdisc", "add", "dev", v.Device, "root", "netem", "delay", delay, jitter, "distribution", "normal")
	cmd.Stdin = strings.NewReader("some input")
	var outAdd bytes.Buffer
	cmd.Stdout = &outAdd
	if err := cmd.Run(); err != nil {
		return fmt.Sprintf("Error at qdisc add: %s", err.Error()), nil
	}
	// Return the output of the qdisc commands
	return strings.Join([]string{
		fmt.Sprintf("Outbound delay of %s ms (jitter %s) added", msDelay, msJitter),
		header,
		outDel.String(),
		outAdd.String(),
	}, "\n"), remainder
}

// Add delay in the outbound direction
func (v *VLAN) ReplyToOut(msg *tgbotapi.Message, fields []string) (string, []string) {
	data, remainder := v.getParams(msg, fields)
	if remainder == nil {
		return data.err, remainder
	}
	return v.delay(v.Device, data.delay, data.jitter, remainder)
}
