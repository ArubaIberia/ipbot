package main

import (
	"fmt"
	"net"
	"strings"

	"gopkg.in/telegram-bot-api.v4"
)

type Interfaces struct {
	Current map[string][]net.IP
}

// Reply to a message asking for IP Addresses.
func (ifaces *Interfaces) ReplyToIP(msg *tgbotapi.Message, fields []string) (string, []string) {
	if err := ifaces.Update(); err != nil {
		return err.Error(), nil
	}
	return ifaces.ToString(), fields[1:]
}

// Converts an ifaceMap to string
func (ifaces *Interfaces) ToString() string {
	lines := make([]string, 0, len(ifaces.Current))
	for name, ips := range ifaces.Current {
		texts := make([]string, 0, len(ips))
		for _, ip := range ips {
			texts = append(texts, ip.String())
		}
		line := fmt.Sprintf("%s: %s", name, strings.Join(texts, ", "))
		lines = append(lines, line)
	}
	return strings.Join(lines, "\n")
}

// Enumerates all interfaces and IP Addresses
func (ifaces *Interfaces) Update() error {
	netif, err := net.Interfaces()
	result := make(map[string][]net.IP)
	if err != nil {
		return err
	}
	for _, i := range netif {
		addrs, err := i.Addrs()
		if err != nil {
			return err
		}
		ips := make([]net.IP, 0, len(addrs))
		for _, addr := range addrs {
			var ip net.IP
			switch v := addr.(type) {
			case *net.IPNet:
				ip = v.IP
			case *net.IPAddr:
				ip = v.IP
			}
			if ipv4 := ip.To4(); ipv4 != nil {
				ips = append(ips, ipv4)
			}
		}
		if len(ips) > 0 {
			result[i.Name] = ips
		}
	}
	ifaces.Current = result
	return nil
}
