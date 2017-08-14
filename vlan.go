package main

import (
	"bytes"
	"errors"
	"fmt"
	"os/exec"
	"regexp"
	"strconv"
	"strings"

	"gopkg.in/telegram-bot-api.v4"
)

// RegisterVLAN adds "vlan", "in", "out" commands to bot
func RegisterVLAN(bot Bot, ifaces *Interfaces) {
	ifaces.Update()
	v := &vlan{Interfaces: ifaces}
	bot.Add("vlan", func(bot Bot, msg *tgbotapi.Message, tokens *Tokens) string {
		return v.replyToVLAN(bot, msg, tokens)
	})
	bot.Add("in", func(bot Bot, msg *tgbotapi.Message, tokens *Tokens) string {
		return v.replyToIn(bot, msg, tokens)
	})
	bot.Add("out", func(bot Bot, msg *tgbotapi.Message, tokens *Tokens) string {
		return v.replyToOut(bot, msg, tokens)
	})
}

// VLAN data
type vlan struct {
	Selected   int         // Currently selected vlan
	Interfaces *Interfaces // Enumeration of interfaces
	Device     string      // Device name for selected VLAN
	IFB        string      // IFB device name for selected vlan
}

// Impairment parameters
type params struct {
	delay, jitter     int
	loss, correlation float64
}

// ReplyToVLAN selects a particular VLAN
func (v *vlan) replyToVLAN(bot Bot, msg *tgbotapi.Message, tokens *Tokens) string {
	if tokens.Remaining() < 1 {
		return "Error: must provide the VLAN number (vlan <vlan_number>)"
	}
	vlan, err := strconv.Atoi(tokens.Next())
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
	ifb, err := v.getIFB()
	if err != nil {
		return fmt.Sprintf("Could not get IFB: %s", err.Error())
	}
	v.IFB = ifb
	return fmt.Sprintf("VLAN %d selected", vlan)
}

// ReplyToIn adds delay in the outbound direction
func (v *vlan) replyToIn(bot Bot, msg *tgbotapi.Message, tokens *Tokens) string {
	if v.IFB == "" {
		return "Current VLAN does not have IFB device assigned"
	}
	params, err := v.getParams(msg, tokens)
	if err != nil {
		return err.Error()
	}
	return v.impair(v.IFB, params)
}

// ReplyToOut adds delay in the outbound direction
func (v *vlan) replyToOut(bot Bot, msg *tgbotapi.Message, tokens *Tokens) string {
	params, err := v.getParams(msg, tokens)
	if err != nil {
		return err.Error()
	}
	return v.impair(v.Device, params)
}

// Get Delay, Jitter, PL and PL correlation from command
func (v *vlan) getParams(msg *tgbotapi.Message, tokens *Tokens) (params, error) {
	result := params{}
	if v.Selected == 0 {
		return result, errors.New("No VLAN selected. Run \"vlan\" for more info")
	}
	if tokens.Remaining() <= 0 {
		return result, errors.New("Error: must at least provide delay (ms). Format: [in|out] <delay_ms> <jitter_ms> <PL %> <correlation %>")
	}
	msDelay, err := strconv.Atoi(tokens.Next())
	if err != nil {
		return result, fmt.Errorf("delay is not an int: %s", err.Error())
	}
	if msDelay < 0 || msDelay > 4094 {
		return result, errors.New("Error: Delay must be between 0 and 4094 milliseconds")
	}
	result.delay = msDelay
	if tokens.Remaining() > 0 {
		msJitter, err := strconv.Atoi(tokens.Next())
		if err != nil {
			tokens.Back()
			return result, nil
		}
		if msJitter < 0 || msJitter > 4094 {
			return result, errors.New("Error: Jitter must be between 0 and 4094 milliseconds")
		}
		result.jitter = msJitter
	}
	if tokens.Remaining() > 0 {
		pl, err := strconv.ParseFloat(tokens.Next(), 32)
		if err != nil {
			tokens.Back()
			return result, nil
		}
		if pl < 0 || pl > 100 {
			return result, errors.New("Error: Packet loss must be between 0.0 and 100.0 percent")
		}
		result.loss = pl
	}
	if tokens.Remaining() > 0 {
		corr, err := strconv.ParseFloat(tokens.Next(), 32)
		if err != nil {
			tokens.Back()
			return result, nil
		}
		if corr < 0 || corr > 100 {
			return result, errors.New("Error: Correlation must be between 0.0 and 100.0 percent")
		}
		result.correlation = corr
	}
	return result, nil
}

// Add impairments (delay, jitter, loss...) to an interface
func (v *vlan) impair(iface string, p params) string {
	messages := make([]string, 0, 10)
	// Remove any qdisc
	cmd := exec.Command("tc", "qdisc", "del", "dev", iface, "root")
	var outDel bytes.Buffer
	cmd.Stdout = &outDel
	if err := cmd.Run(); err != nil {
		messages = append(messages, fmt.Sprintf("(Ignore) Error at qdisc del: %s", err.Error()))
	}
	messages = append(messages, fmt.Sprintf("Cleared interface %s", iface))
	messages = append(messages, outDel.String())
	// Prepare for adding jitter and packet loss
	cmdLine := fmt.Sprintf("tc qdisc add dev %s root netem", iface)
	doApply := false
	if p.delay != 0 {
		doApply = true
		cmdLine = fmt.Sprintf("%s delay %dms", cmdLine, p.delay)
		if p.jitter != 0 {
			cmdLine = fmt.Sprintf("%s %dms distribution normal", cmdLine, p.jitter)
		}
	}
	if p.loss != 0 {
		doApply = true
		cmdLine = fmt.Sprintf("%s loss %f%%", cmdLine, p.loss)
		if p.correlation != 0 {
			cmdLine = fmt.Sprintf("%s %f%%", cmdLine, p.correlation)
		}
	}
	// If delay != 0, add it
	var outAdd bytes.Buffer
	if doApply {
		messages = append(messages, fmt.Sprintf("Policy for interface %s: %dms delay (%dms jitter), %f%% PL (%f%% correlation)", iface, p.delay, p.jitter, p.loss, p.correlation))
		fields := strings.Fields(cmdLine)
		cmd = exec.Command(fields[0], fields[1:]...)
		cmd.Stdout = &outAdd
		if err := cmd.Run(); err != nil {
			messages = append(messages, fmt.Sprintf("Error at qdisc add: %s", err.Error()))
		}
		messages = append(messages, outAdd.String())

	}
	// Return the output of the qdisc commands
	return strings.Join(messages, "\n")
}

// Gets the IFB interface associated to the selected VLAN
func (v *vlan) getIFB() (string, error) {
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
