package main

import (
	"fmt"
	"io"
	"log"
	"os"
	"path"
	// "path/filepath"
	// "strings"
)

func changeDirectory(path string) {
	if err := os.Chdir(path); err != nil {
		log.Fatalf("Failed to change directory to %s: %s\n", path, err)
	}
}

func start() uint8 {
	startingWorkingDirectory, err := os.Getwd()
	if err != nil {
		log.Fatalf("Failed to get the current working directory: %s\n", err)
	}

	tempDirPath, err := os.MkdirTemp("", "cd-rip")
	if err != nil {
		log.Fatalf("Failed to make temporary directory: %s\n", err)
	}
	defer os.RemoveAll(tempDirPath)

	log.Printf("Created temp directory %s\n", tempDirPath)

	if err = os.Chdir(tempDirPath); err != nil {
		log.Printf("Failed to do change current working directory to %s\n", tempDirPath)
		log.Printf("Aborting...")
		return 1
	}

	defer changeDirectory(startingWorkingDirectory)

	if err := RipCD(tempDirPath); err != nil {
		log.Printf("Failed to rip CD: %s\n", err)
		changeDirectory(startingWorkingDirectory)
		os.Exit(1)
	}

	metadata, err := GetMetaDataForCD()
	if err != nil {
		log.Printf("Failed to get meta data from CD: %s\n", err)
		changeDirectory(startingWorkingDirectory)
		return 1
	}

	songs := GetFlacTags(metadata)

	for song, tags := range songs {
		trackName := fmt.Sprintf("track%02d.flac", tags.TrackNumber)
		oldPath := path.Join(tempDirPath, trackName)
		newFileName := fmt.Sprintf("%s-no-tags.flac", song)
		newPath := path.Join(tempDirPath, newFileName)
		if err := os.Rename(oldPath, newPath); err != nil {
			log.Printf("Failed to rename %s to %s: %s\n", oldPath, newPath, err)
			changeDirectory(startingWorkingDirectory)
			return 1
		}
	}

	AddFLACTags(songs)

	artistName := metadata.Disc.Releases.Release[0].AristCredit.NameCredit[0].Artist.Name
	albumName := metadata.Disc.Releases.Release[0].Title
	pathToAlbum := path.Join("/Volumes", "Cloud 1", "music", artistName, albumName)
	if err = os.MkdirAll(pathToAlbum, 777); err != nil {
		log.Printf("Failed to create directory %s: %s\n", pathToAlbum, err)
		changeDirectory(startingWorkingDirectory)
		return 1
	}
	for song := range songs {
		fileName := fmt.Sprintf("%s.flac", song)
		oldPath := path.Join(tempDirPath, fileName)
		newPath := path.Join(pathToAlbum, fileName)

		songFile, err := os.Open(fileName)
		if err != nil {
			log.Printf("Failed to open file %s: %s\n", oldPath, err)
			changeDirectory(startingWorkingDirectory)
			return 1
		}

		destSongFile, err := os.Create(newPath)
		if err != nil {
			log.Printf("Failed to create dest song file %s: %s\n", newPath, err)
			changeDirectory(startingWorkingDirectory)
			return 1
		}

		_, err = io.Copy(destSongFile, songFile)
		if err != nil {
			log.Printf("Failed to write song %s to %s: %s\n", oldPath, newPath, err)
			changeDirectory(startingWorkingDirectory)
			return 1
		}

		songFile.Close()
		destSongFile.Close()
	}

	return 0
}

func main() {
	code := start()
	os.Exit(int(code))
}
