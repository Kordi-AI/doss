package main

import (
	"flag"
	"fmt"

	"github.com/Kordi-AI/doss/internal/gitx"
	"github.com/Kordi-AI/doss/internal/vault"
)

func cmdDevices(args []string) error {
	fs := flag.NewFlagSet("devices", flag.ExitOnError)
	if err := fs.Parse(args); err != nil {
		return err
	}
	d, err := vault.MustExist()
	if err != nil {
		return err
	}
	if fs.NArg() > 0 {
		if fs.NArg() != 2 || fs.Arg(0) != "unregister" {
			return fmt.Errorf("usage: doss devices [unregister <device-id>]")
		}
		return unregisterDevice(d, fs.Arg(1))
	}
	return printDevices(d)
}

func cmdUnregister(args []string) error {
	fs := flag.NewFlagSet("unregister", flag.ExitOnError)
	if err := fs.Parse(args); err != nil {
		return err
	}
	d, err := vault.MustExist()
	if err != nil {
		return err
	}
	switch fs.NArg() {
	case 0:
		id, err := chooseDeviceToUnregister(d)
		if err != nil {
			return err
		}
		return unregisterDevice(d, id)
	case 1:
		return unregisterDevice(d, fs.Arg(0))
	default:
		return fmt.Errorf("usage: doss unregister [device-id]")
	}
}

func chooseDeviceToUnregister(dir string) (string, error) {
	if !stdinIsTTY() {
		return "", fmt.Errorf("usage: doss unregister [device-id]; run `doss unregister` in a terminal to choose a device, or pass a device id for non-interactive use")
	}
	current := vault.DeviceID(dir)
	devices, err := vault.Devices(dir)
	if err != nil {
		return "", err
	}
	var candidates []vault.Device
	var options []string
	for _, dev := range devices {
		if dev.ID == current || dev.Status != "active" {
			continue
		}
		candidates = append(candidates, dev)
		label := dev.Label
		if label == "" {
			label = "(no label)"
		}
		options = append(options, fmt.Sprintf("%s  %s", label, dev.ID))
	}
	if len(candidates) == 0 {
		return "", fmt.Errorf("no other active devices are registered")
	}
	choice := newPrompter().choose("Select a device to unregister:", options...)
	return candidates[choice].ID, nil
}

func unregisterDevice(dir, id string) error {
	if id == vault.DeviceID(dir) {
		return fmt.Errorf("use `doss uninstall` to unregister the current device")
	}
	if dirty, err := gitx.Dirty(dir); err != nil {
		return err
	} else if dirty {
		return fmt.Errorf("vault has uncommitted changes; run `doss sync` before unregistering another device")
	}
	devices, err := vault.Devices(dir)
	if err != nil {
		return err
	}
	found := false
	for _, dev := range devices {
		if dev.ID == id {
			found = true
			break
		}
	}
	if !found {
		return fmt.Errorf("device %q is not registered", id)
	}
	if revoked, err := revokeDeviceDeployKey(dir, id); err != nil {
		return err
	} else if revoked {
		fmt.Printf("✓ GitHub deploy key revoked for %s\n", id)
	} else {
		fmt.Printf("warning: no GitHub deploy key recorded for %s; only the synced device registry will be updated\n", id)
	}
	if _, err := vault.UnregisterDevice(dir, id); err != nil {
		return err
	}
	if err := syncGit(dir, "doss: unregister device "+id, false); err != nil {
		return err
	}
	return printDevices(dir)
}

func printDevices(dir string) error {
	current := vault.DeviceID(dir)
	devices, err := vault.Devices(dir)
	if err != nil {
		return err
	}
	if len(devices) == 0 {
		fmt.Println("devices: none registered yet")
		return nil
	}
	active := 0
	for _, dev := range devices {
		if dev.Status == "active" {
			active++
		}
	}
	fmt.Printf("devices: %d active / %d total\n", active, len(devices))
	for _, dev := range devices {
		mark := " "
		if dev.ID == current {
			mark = "*"
		}
		fmt.Printf("  %s %-20s %-12s %s\n", mark, dev.ID, dev.Status, dev.Label)
	}
	return nil
}
