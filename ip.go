package main

import (
	"fmt"
	"net"
	"strings"

	"gopkg.in/telegram-bot-api.v4"
)

func replyToIP(msg *tgbotapi.Message) string {
	ifaces, err := getIPs()
	if err != nil {
		return err.Error()
	}
	return ifaces.ToString()
}

type ifaceMap map[string][]net.IP

func (ifaces ifaceMap) ToString() string {
	lines := make([]string, 0, len(ifaces))
	for name, ips := range ifaces {
		texts := make([]string, 0, len(ips))
		for _, ip := range ips {
			texts = append(texts, ip.String())
		}
		line := fmt.Sprintf("%s: %s", name, strings.Join(texts, ", "))
		lines = append(lines, line)
	}
	return strings.Join(lines, "\n")
}

func getIPs() (ifaceMap, error) {
	result := make(ifaceMap)
	ifaces, err := net.Interfaces()
	if err != nil {
		return nil, err
	}
	for _, i := range ifaces {
		addrs, err := i.Addrs()
		if err != nil {
			return nil, err
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
	return result, nil
}
