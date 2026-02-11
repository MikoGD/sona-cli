package main

import (
	"bufio"
	"bytes"
	"errors"
	"fmt"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/mikogd/maokai"
)

// Calls udevadm info to get the name of the device
//
// Command exeucted `udevadm info --query=property --name=<dev>`
func getDevInfo(dev string) ([]byte, error) {
	cmd := exec.Command("udevadm", "info", "--query=property", "--name="+dev)
	out, err := cmd.Output()

	var exitError *exec.ExitError
	if errors.As(err, &exitError) {
		errorMessage := fmt.Sprintf(
			"Error running \"udevadm info --query=property --name=%s\" command:\n%v", dev, exitError.Stderr)
		return []byte{}, errors.New(errorMessage)
	}

	if err != nil {
		errorMessage := fmt.Sprintf("Error running \"udevadm info --query=property --name=%s\" command:\n%v", dev, err)
		return []byte{}, errors.New(errorMessage)
	}

	return out, nil
}

func checkIsCDROM(dev string) (bool, error) {
	out, err := getDevInfo(dev)
	if err != nil {
		errorMessage := fmt.Sprintf("Error getting CD device\n%v", err)
		return false, errors.New(errorMessage)
	}

	scanner := bufio.NewScanner(bytes.NewReader(out))
	isCDROM := false
	for scanner.Scan() {
		line := scanner.Text()
		if strings.Trim(line, " \t\r\n") == "ID_CDROM_BD=1" {
			return false, nil
		}

		if strings.Trim(line, " \t\r\n") == "ID_CDROM=1" {
			isCDROM = true
		}
	}

	if isCDROM {
		return true, nil
	}

	return false, nil
}

func getCDDriveDeviceName(logger maokai.Logger) (string, error) {
	logger.CreateLog("Getting all device names")
	// Find all /dev/sr* devices
	matches, err := filepath.Glob("/dev/sr*")
	if err != nil {
		errorMessage := fmt.Sprintf("Error listing /dev/sr*: %v", err)
		return "", errors.New(errorMessage)
	}

	// Check which are actual CD/DVD drives
	for _, dev := range matches {
		isCDROM, err := checkIsCDROM(dev)

		if err != nil {
			errorMessage := fmt.Sprintf("Failed to find CD drive\n%v", err)
			return "", errors.New(errorMessage)
		}

		if isCDROM {
			logger.CreateLog(fmt.Sprintf("Found CD drive %s", dev))
			return dev, nil
		}
	}

	return "", errors.New("No devices attached were CD drives")
}

func sanitizeSongName(logger maokai.Logger, songName string) string {
	pattern := `[\/\\:*?"<>|&!;#~%^\[\]{}()$=@,.` + "`" + `'\t\n\r]`
	re := regexp.MustCompile(pattern)
	// Replace all matched characters with underscore
	sanitizedSongName := re.ReplaceAllString(songName, "_")

	logger.CreateLogf("Sanitized \"%s\" to \"%s\"", songName, sanitizedSongName)

	return sanitizedSongName
}
