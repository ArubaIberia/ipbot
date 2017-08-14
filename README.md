IPBOT: telegram chat bot to manage the raspi WAN emulator
=========================================================

A simple chat bot to manage the RaspberryPI-based WAN emulator (see https://github.com/rafahpe/piwem).

This bot accepts a few commands:

- **ip**: Enumerate the IP addresses of the device.
- **vlan <VLAN number>**: Select a particular VLAN to apply WAN impairments.
- **in <delay ms> <jitter ms> <packet loss %> <PL correlation %>**: Apply the specified VLAN impairments to traffic entering the raspberry PI through the previously selected VLAN interface.
- **out <delay ms> <jitter ms> <packet loss %> <PL correlation %>**: Apply the specified VLAN impairments to traffic leaving the raspberry PI through the previously selected VLAN interface.

To run the bot, you have to provide the telegram API key, either:

- In the command line, with the "-token" parameter.
- In the environment variable IPBOT_API_KEY
