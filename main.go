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
)

func main() {
	if !strings.HasSuffix(os.Args[1], ".blurl") {
		fmt.Println("input must be a blurl")
		return
	}

	blurl, err := parseBLURL(string(os.Args[1]))

	if err != nil {
		fmt.Println(err)
	}

	mediaurl := GetMediaURL(blurl)

	decodedEV, err := base64.StdEncoding.DecodeString(blurl.Ev)
	if err != nil {
		fmt.Println("Error decoding base64:", err)
		return
	}

	parsedev, err := blurldecrypt.ParseEV(decodedEV)

	key := blurldecrypt.GetEncryptionKey("keys.bin", parsedev.Nonce, parsedev.Key[:])

	if key == nil {
		return
	}

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
			err := HandleDownloadTrack(fmt.Sprintf("master_%s", adaptation.ContentType), numberOfSegments, getBaseURL(mediaurl), strings.ReplaceAll(adaptation.Representation[0].SegmentTemplate.Initialization, "$RepresentationID$", adaptation.Representation[0].ID), adaptation.Representation[0].ID, hex.EncodeToString(key))

			if err != nil {
				fmt.Println("Error Downloading Track", err)
				return
			}
		}

		/*Scuffed but works */
		if len(mpddata.Period.AdaptationSet) == 2 {
			Merge("master_video.mp4", "master_audio.mp4")
		}

	} else {
		fmt.Println("Invalid number of track segments! exiting.")
		return
	}
}
