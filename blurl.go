package main

import (
	"bufio"
	"bytes"
	"compress/zlib"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"os"
	"strconv"
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
	fmt.Println("Available playlists:")
	for i, playlist := range blurl.Playlists {
		fmt.Printf("%d: %s\n", i+1, playlist.Language)
	}
	fmt.Print("Enter the number of your preferred playlist: ")

	scanner := bufio.NewScanner(os.Stdin)
	if scanner.Scan() {
		input := scanner.Text()
		choice, err := strconv.Atoi(input)
		if err != nil {
			fmt.Println("Invalid input, please enter a number")
			return ""
		}
		if choice < 1 || choice > len(blurl.Playlists) {
			fmt.Println("Selected number is out of range")
			return ""
		}
		return blurl.Playlists[choice-1].URL
	} else {
		fmt.Println("Failed to read input")
		return ""
	}
}

func parseBLURLFromJSON(inblurl *BLURL, filepath string) error {
	file, err := os.Open(filepath)
	if err != nil {
		return err
	}
	defer file.Close()

	err = json.NewDecoder(file).Decode(&inblurl)
	if err != nil {
		log.Fatalf("Error decoding JSON: %v", err)
	}

	return nil
}

func parseBLURL(inblurl *BLURL, filepath string) error {
	file, err := os.Open(filepath)
	if err != nil {
		return err
	}
	defer file.Close()

	_, err = file.Seek(8, 0)
	if err != nil {
		return err
	}

	decompressedData, err := decompressData(file)
	if err != nil {
		return err
	}

	err = json.Unmarshal(decompressedData, &inblurl)
	if err != nil {
		return err
	}

	return nil
}
