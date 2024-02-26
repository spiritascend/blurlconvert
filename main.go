package main

import (
	"blurlconvert/blurldecrypt"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"math"
	"os"
	"strconv"
	"strings"
	"time"
)

func main() {
	if len(os.Args) <= 2 {
		return
	}

	if !strings.HasSuffix(os.Args[1], ".blurl") && !strings.HasSuffix(os.Args[1], ".json") {
		fmt.Println("input must be a blurl or a json")
		return
	}

	var blurl BLURL

	if strings.HasSuffix(os.Args[1], ".blurl") {
		err := parseBLURL(&blurl, string(os.Args[1]))

		if err != nil {
			fmt.Println(err)
			return
		}
	} else {
		err := parseBLURLFromJSON(&blurl, string(os.Args[1]))

		if err != nil {
			fmt.Println(err)
			return
		}
	}

	var mediaurl string
	if len(blurl.Playlists) == 1 {
		mediaurl = blurl.Playlists[0].URL
	} else {
		mediaurl = GetMediaURL(&blurl)
	}

	var key []byte

	if len(blurl.Ev) > 0 {
		decodedEV, err := base64.StdEncoding.DecodeString(blurl.Ev)
		if err != nil {
			fmt.Println("Error decoding base64:", err)
			return
		}

		parsedev, err := blurldecrypt.ParseEV(decodedEV)

		key = blurldecrypt.GetEncryptionKey("keys.bin", parsedev.Nonce, parsedev.Key[:])

		if key == nil {
			return
		}

		fmt.Printf("Key: %02x\n", key)
	}

	mediaurl, err := RemoveDuplicateUUIDPath(mediaurl)

	mpddata, err := GetPlaylistMetadataByID(mediaurl)

	if err != nil {
		fmt.Println("Error getting playlist metadata:", err)
		return
	}

	trackduration := GetPlaylistDuration(mpddata)

	if trackduration < 0 {
		fmt.Println("Track Duration is 0 exiting!")
		return
	}

	segmentDurationStr := mpddata.Period.AdaptationSet[0].Representation[0].SegmentTemplate.Duration
	segmentDuration, err := strconv.ParseInt(segmentDurationStr, 10, 64)
	if err != nil {
		fmt.Println("Error parsing segment duration:", err)
		return
	}
	timescaleStr := mpddata.Period.AdaptationSet[0].Representation[0].SegmentTemplate.Timescale
	timescale, err := strconv.ParseInt(timescaleStr, 10, 64)
	if err != nil {
		fmt.Println("Error parsing timescale:", err)
		return
	}

	numberOfSegments := math.Ceil(trackduration / (float64(segmentDuration) / float64(timescale)))

	if numberOfSegments > 0 {
		fmt.Printf("===================================================================================\n")
		fmt.Printf("Track Segments: %.0f\n", numberOfSegments)
		fmt.Printf("Media Type: %s\n", mpddata.Period.AdaptationSet[0].ContentType)
		fmt.Printf("Media Codec: %s\n", mpddata.Period.AdaptationSet[0].Representation[0].Codecs)
		fmt.Printf("Sample Rate: %skHz\n", mpddata.Period.AdaptationSet[0].Representation[0].AudioSamplingRate)
		fmt.Printf("===================================================================================\n")

		for _, adaptation := range mpddata.Period.AdaptationSet {
			err := HandleDownloadTrack(adaptation.ContentType, fmt.Sprintf("master_%s", adaptation.ContentType), numberOfSegments, getBaseURL(mediaurl), strings.ReplaceAll(adaptation.Representation[0].SegmentTemplate.Initialization, "$RepresentationID$", adaptation.Representation[0].ID), adaptation.Representation[0].ID, hex.EncodeToString(key))

			if err != nil {
				fmt.Println("Error Downloading Track", err)
				return
			}
		}

		/*Scuffed but works */
		if len(mpddata.Period.AdaptationSet) == 2 {
			Merge("master_video.mp4", "master_audio.mp4", EncodeToBase62(mpddata.Period.AdaptationSet[0].ContentProtection[0].DefaultKID)[:8])
		}

		time.Sleep(1 * time.Second)

		os.RemoveAll("./downloads")

	} else {
		fmt.Println("Invalid number of track segments! exiting.")
		return
	}
}
