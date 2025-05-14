package main

import (
	"errors"
	"fmt"
	"log"
	"os"
	"os/exec"
	"strings"
)

func convertToFlac(fileName string) error {
  // ffmpeg -i track01.cdda.wav -c:a flac -compression_level 12 track01.flac
  fileNameTokens := strings.Split(fileName, ".")
  if len(fileNameTokens) != 3 {
    errorMessage := fmt.Sprintf("Split file name contained less than 3 items: %v\n", fileNameTokens)
    return errors.New(errorMessage)
  }

  newFileName := fmt.Sprintf("%s.flac", fileNameTokens[0])
  cmd := exec.Command("ffmpeg", "-i", fileName, "-c:a", "flac", "-compression_level", "12", newFileName)
  cmd.Stdout = os.Stdout
  cmd.Stderr = os.Stderr

  log.Printf("Converting %s to %s\n", fileName, newFileName)
  if err := cmd.Run(); err != nil {
    return err
  }
  log.Printf("Finished %s to %s\n", fileName, newFileName)

  return nil
}

func RipCD(destPath string) error {
  cmd := exec.Command("cdparanoia", "-Bw")
  cmd.Stdout = os.Stdout
  cmd.Stderr = os.Stderr

  if err := cmd.Run(); err != nil {
    errorMessage := fmt.Sprintf("Failed to run cdparanoia -Bw in %s: %s\n", destPath, err)
    return errors.New(errorMessage)
  }

  currentDirectory, err := os.ReadDir(".")
  if err != nil {
    errorMessage := fmt.Sprintf("Failed to read current directory: %s\n", err)
    return errors.New(errorMessage)
  }

  for _, entry := range currentDirectory {
    if entry.IsDir() {
      continue
    }

    if !strings.Contains(entry.Name(), ".cdda.wav") {
      continue
    }

    if err := convertToFlac(entry.Name()); err != nil {
      errorMessage := fmt.Sprintf("Failed to convert %s to flac: %s\n", entry.Name(), err)
      return errors.New(errorMessage)
    }
  }

  return nil
}
