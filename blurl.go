package main

import (
	"bufio"
	"bytes"
	"compress/zlib"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"
)

type Playlist struct {
	Data     string  `json:"data"`
	Duration float64 `json:"duration"`
	Language string  `json:"language"`
	Type     string  `json:"type"`
	URL      string  `json:"url"`
}

type BLURL struct {
	AudioOnly bool       `json:"audioonly"`
	Ev        string     `json:"ev"`
	PartySync bool       `json:"partysync"`
	Playlists []Playlist `json:"playlists"`
	Type      string     `json:"type"`
}

func decompressData(data io.Reader) ([]byte, error) {
	var decompressedData bytes.Buffer
	decompressor, err := zlib.NewReader(data)
	if err != nil {
		return nil, err
	}
	defer decompressor.Close()

	_, err = io.Copy(&decompressedData, decompressor)
	if err != nil {
		return nil, err
	}

	return decompressedData.Bytes(), nil
}

func GetMediaURL(blurl *BLURL) string {
	fmt.Println("Available languages:")
	for _, playlist := range blurl.Playlists {
		fmt.Println("- " + playlist.Language)
	}
	fmt.Print("Enter your preferred language: ")

	scanner := bufio.NewScanner(os.Stdin)
	if scanner.Scan() {
		chosenLanguage := scanner.Text()

		for _, playlist := range blurl.Playlists {
			if strings.EqualFold(playlist.Language, chosenLanguage) {
				return playlist.URL
			}
		}
		fmt.Println("No playlist found for the selected language")
	} else {
		fmt.Println("Failed to read input")
	}
	return ""
}

func parseBLURL(filepath string) (*BLURL, error) {
	file, err := os.Open(filepath)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	_, err = file.Seek(8, 0)
	if err != nil {
		return nil, err
	}

	decompressedData, err := decompressData(file)
	if err != nil {
		return nil, err
	}

	var jsonblurl BLURL
	err = json.Unmarshal(decompressedData, &jsonblurl)
	if err != nil {
		return nil, err
	}

	return &jsonblurl, nil
}
