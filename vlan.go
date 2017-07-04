package main

import (
	"bytes"
	"fmt"
	"os/exec"
	"regexp"
	"strconv"
	"strings"

	"gopkg.in/telegram-bot-api.v4"
)

type VLAN struct {
	Selected   int
	Interfaces *Interfaces
	Device     string
	IFB        string
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
	ifb, err := v.getIFB()
	if err != nil {
		return fmt.Sprintf("Could not get IFB: %s", err.Error()), fields[2:]
	}
	v.IFB = ifb
	return fmt.Sprintf("VLAN %d selected", vlan), fields[2:]
}

type params struct {
	delay, jitter     int
	loss, correlation float64
	err               string
}

// Get Delay, Jitter, PL and PL correlation from command
func (v *VLAN) getParams(msg *tgbotapi.Message, fields []string) (params, []string) {
	if v.Selected == 0 {
		return params{err: "No VLAN selected. Run \"vlan\" for more info"}, nil
	}
	if len(fields) < 2 {
		return params{err: "Error: must at least provide delay (ms). Format: [in|out] <delay_ms> <jitter_ms> <PL %> <correlation %>"}, nil
	}
	result := params{}
	spent := 2
	msDelay, err := strconv.Atoi(fields[1])
	if err != nil {
		return params{err: fmt.Sprintf("delay is not an int: %s", err.Error())}, nil
	}
	if msDelay < 1 || msDelay > 4094 {
		return params{err: "Error: Delay must be between 1 and 4094 milliseconds"}, nil
	}
	result.delay = msDelay
	if len(fields) > 2 {
		if msJitter, err := strconv.Atoi(fields[2]); err == nil {
			if msJitter < 1 || msJitter > 4094 {
				return params{err: "Error: Delay must be between 1 and 4094 milliseconds"}, nil
			}
			result.jitter = msJitter
			spent = 3
			if len(fields) > 3 {
				if pl, err := strconv.ParseFloat(fields[3], 32); err == nil {
					if pl < 0 || pl > 100 {
						return params{err: "Error: Packet loss must be between 0.0 and 100.0 percent"}, nil
					}
					result.loss = pl
					spent = 4
					if len(fields) > 4 {
						if corr, err := strconv.ParseFloat(fields[4], 32); err == nil {
							if corr < 0 || corr > 100 {
								return params{err: "Error: Correlation must be between 0.0 and 100.0 percent"}, nil
							}
							result.correlation = corr
							spent = 5
						}
					}
				}
			}
		}
	}
	return result, fields[spent:]
}

// Add impairments (delay, jitter, loss...) to an interface
func (v *VLAN) impair(iface string, p params, remainder []string) (string, []string) {
	// Remove any qdisc
	cmd := exec.Command("tc", "qdisc", "del", "dev", iface, "root")
	var outDel bytes.Buffer
	cmd.Stdout = &outDel
	header := ""
	if err := cmd.Run(); err != nil {
		header = fmt.Sprintf("(Ignore) Error at qdisc del: %s", err.Error())
	}
	// Prepare for adding jitter and oacket loss
	cmdLine := fmt.Sprintf("tc qdisc add dev %s root netem", iface)
	if p.delay != 0 {
		cmdLine = fmt.Sprintf("%s delay %dms", cmdLine, p.delay)
		if p.jitter != 0 {
			cmdLine = fmt.Sprintf("%s %dms distribution normal", cmdLine, p.jitter)
		}
	}
	if p.loss != 0 {
		cmdLine = fmt.Sprintf("%s loss %f%%", cmdLine, p.loss)
		if p.correlation != 0 {
			cmdLine = fmt.Sprintf("%s %f%%", cmdLine, p.correlation)
		}
	}
	fields := strings.Fields(cmdLine)
	cmd = exec.Command(fields[0], fields[1:]...)
	var outAdd bytes.Buffer
	cmd.Stdout = &outAdd
	if err := cmd.Run(); err != nil {
		return fmt.Sprintf("Error at qdisc add: %s", err.Error()), nil
	}
	// Return the output of the qdisc commands
	return strings.Join([]string{
		fmt.Sprintf("Policy for interface %s: %dms delay (%dms jitter), %f%% PL (%f%% correlation)", p.delay, p.jitter, p.loss, p.correlation),
		header,
		outDel.String(),
		outAdd.String(),
	}, "\n"), remainder
}

func (v *VLAN) getIFB() (string, error) {
	cmd := exec.Command("tc", "filter", "show", "dev", v.Device, "root")
	var outShow bytes.Buffer
	cmd.Stdout = &outShow
	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("Error at filter show: %s", err.Error())
	}
	data := outShow.String()
	re := regexp.MustCompile("Egress Redirect to device ifb[0-9]")
	match := re.FindString(data)
	if match == "" {
		return "", fmt.Errorf("Missing IFB device for %s in %s", v.Device, data)
	}
	ifbFields := strings.Fields(match)
	return ifbFields[len(ifbFields)-1], nil
}

// Add delay in the outbound direction
func (v *VLAN) ReplyToOut(msg *tgbotapi.Message, fields []string) (string, []string) {
	data, remainder := v.getParams(msg, fields)
	if remainder == nil {
		return data.err, remainder
	}
	return v.impair(v.Device, data, remainder)
}

// Add delay in the outbound direction
func (v *VLAN) ReplyToIn(msg *tgbotapi.Message, fields []string) (string, []string) {
	if v.IFB == "" {
		return "Current VLAN does not have IFB device assigned", nil
	}
	data, remainder := v.getParams(msg, fields)
	if remainder == nil {
		return data.err, remainder
	}
	return v.impair(v.IFB, data, remainder)
}
