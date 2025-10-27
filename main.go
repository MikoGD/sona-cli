package main

import (
	"fmt"
	"log"
	"os"
	"path"
	"path/filepath"
	"strconv"

	"github.com/mikogd/hextech/env"
)

func changeDirectory(path string) {
	if err := os.Chdir(path); err != nil {
		log.Fatalf("Failed to change directory to %s: %s\n", path, err)
	}
}

func start() uint8 {
	env.LoadEnv("./.env")

	metadata, err := GetMetaDataForCD()
	if err != nil {
		log.Printf("Failed to get meta data: %v\n", err)
		return 1
	}

	release := GetRelease(metadata)

	pathToMusicFolder := os.Getenv("PATH_TO_DEST_MUSIC")
	if pathToMusicFolder == "" {
		log.Fatalf("Failed to get PATH_TO_DEST_MUSIC environment variable")
	}

	artistName := release.AristCredit.NameCredit[0].Artist.Name
	albumName := release.Title
	pathToAlbum := path.Join(pathToMusicFolder, artistName, sanitizeSongName(albumName))
	if _, err := os.Stat(pathToAlbum); os.IsNotExist(err) {
		log.Printf("%s doesn't exist, creating directory\n", pathToAlbum)
		if err = os.MkdirAll(pathToAlbum, 0777); err != nil {
			log.Printf("Failed to create directory %s: %s\n", pathToAlbum, err)
			return 1
		}
	} else if err != nil {
		// Some other error while trying to check the folder
		log.Printf("Error checking directory %s: %s\n", pathToAlbum, err)
		return 1
	}

	startingWorkingDirectory, err := os.Getwd()
	if err != nil {
		log.Fatalf("Failed to get the current working directory: %s\n", err)
	}

	if err = os.Chdir(pathToAlbum); err != nil {
		log.Printf("Failed to do change current working directory to %s\n", pathToAlbum)
		log.Printf("Aborting...")
		return 1
	}

	defer changeDirectory(startingWorkingDirectory)

	if err := RipCD(pathToAlbum); err != nil {
		log.Printf("Failed to rip CD: %s\n", err)
		changeDirectory(startingWorkingDirectory)
		os.Exit(1)
	}

	discNumberArg := ""
	if len(os.Args) == 2 {
		discNumberArg = os.Args[1]
	}

	var discNumber int
	if discNumberArg == "" {
		discNumber = 1
	} else {
		discNumber, err = strconv.Atoi(discNumberArg)
		if err != nil {
			log.Fatalf("Failed to convert discNumberArg '%s' to int", discNumberArg)
		}
	}

	log.Printf("discNumber: %d\n", discNumber)

	songs := GetFlacTags(metadata, uint8(discNumber))

	for _, song := range songs {
		log.Printf("song: %s\ntrack number: %d\ntags: %v\n\n", song.Title, song.TrackNumber,  song)
	}

	for _, song := range songs {
		trackName := fmt.Sprintf("track%02d.flac", song.TrackNumber)
		oldPath := path.Join(pathToAlbum, trackName)

		newFileName := fmt.Sprintf("%s-no-tags.flac", sanitizeSongName(song.Title))
		newPath := path.Join(pathToAlbum, newFileName)
		if err := os.Rename(oldPath, newPath); err != nil {
			log.Printf("Failed to rename %s to %s: %s\n", oldPath, newPath, err)
			changeDirectory(startingWorkingDirectory)
			return 1
		}
	}

	AddFLACTags(songs, metadata, uint8(discNumber))

	log.Printf("Deleting all *.cdda.wav files")
	matches, err := filepath.Glob("*.cdda.wav")
	for _, match := range matches {
		if err := os.Remove(match); err != nil {
			log.Printf("Failed to remove %s\n", match)
		}
	}

	log.Printf("Deleting all *-no-tags.flac files")
	matches, err = filepath.Glob("*-no-tags.flac")
	for _, match := range matches {
		if err := os.Remove(match); err != nil {
			log.Printf("Failed to remove %s\n", match)
		}
	}

	return 0
}

func main() {
	code := start()
	os.Exit(int(code))
}
