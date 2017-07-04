package main

import (
	"fmt"
	"strconv"
	"strings"

	"gopkg.in/telegram-bot-api.v4"
)

type VLAN struct {
	Selected   int
	Interfaces *Interfaces
}

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
	found := false
	for name := range v.Interfaces.Current {
		if strings.HasSuffix(name, suffix) {
			found = true
			break
		}
	}
	if !found {
		return fmt.Sprintf("Error: VLAN %d is not found", vlan)
	}
	v.Selected = vlan
	return fmt.Sprintf("VLAN %d selected", vlan)
}
