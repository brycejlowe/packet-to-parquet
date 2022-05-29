package main

import (
	"math"
	"strconv"
	"strings"
	"time"

	"github.com/xitongsys/parquet-go/types"
)

type Packet struct {
	Source struct {
		Layers struct {
			Timestamp    []string `json:"frame.time_epoch"`
			Method       []string `json:"http.request.method"`
			Uri          []string `json:"http.request.uri"`
			ForwardedFor []string `json:"http.x_forwarded_for"`
			UserAgent    []string `json:"http.user_agent"`
			Referer      []string `json:"http.referer"`
			Data         []string `json:"http.file_data"`
		} `json:"layers"`
	} `json:"_source"`
}

type Parquet struct {
	Filename     string `parquet:"name=filename, type=BYTE_ARRAY, convertedtype=UTF8, encoding=PLAIN_DICTIONARY"`
	Timestamp    string `parquet:"name=timestamp, type=INT96"`
	ForwardedFor string `parquet:"name=forwardedfor, type=BYTE_ARRAY, convertedtype=UTF8, encoding=PLAIN_DICTIONARY"`
	Caller       string `parquet:"name=caller, type=BYTE_ARRAY, convertedtype=UTF8, encoding=PLAIN_DICTIONARY"`
	Useragent    string `parquet:"name=useragent, type=BYTE_ARRAY, convertedtype=UTF8, encoding=PLAIN_DICTIONARY"`
	Referer      string `parquet:"name=referer, type=BYTE_ARRAY, convertedtype=UTF8, encoding=PLAIN_DICTIONARY"`
	Uri          string `parquet:"name=uri, type=BYTE_ARRAY, convertedtype=UTF8, encoding=PLAIN_DICTIONARY"`
	Query        string `parquet:"name=query, type=BYTE_ARRAY, convertedtype=UTF8, encoding=PLAIN_DICTIONARY"`
}

func parseRequest(loggedPacket *Packet, loggedParquet *Parquet) error {
	packetLayers := loggedPacket.Source.Layers

	// caller is an artificial structure based on the first characters up until
	// the / in the user-agent string
	userAgent := extractString(packetLayers.UserAgent)
	caller := strings.SplitN(userAgent, "/", 2)[0]

	// tshark encodes the values as strings, these need to be parsed into floats,
	// the separate the whole number from the decimal to get a time object
	// from the unix timestamp
	timeFloat, _ := strconv.ParseFloat(extractString(packetLayers.Timestamp), 64)
	sec, dec := math.Modf(timeFloat)

	loggedParquet.Timestamp = types.TimeToINT96(time.Unix(int64(sec), int64(dec*(1e9))))
	loggedParquet.ForwardedFor = extractString(packetLayers.ForwardedFor)
	loggedParquet.Caller = caller
	loggedParquet.Useragent = userAgent
	loggedParquet.Referer = extractString(packetLayers.Referer)
	loggedParquet.Uri = extractString(packetLayers.Uri)
	loggedParquet.Query = extractString(packetLayers.Data)

	return nil
}

func extractString(toExtract []string) string {
	if len(toExtract) == 0 {
		return ""
	}

	return toExtract[0]
}
