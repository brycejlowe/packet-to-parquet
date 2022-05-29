package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"os/exec"

	"github.com/jessevdk/go-flags"
	"github.com/xitongsys/parquet-go-source/local"
	"github.com/xitongsys/parquet-go/parquet"
	"github.com/xitongsys/parquet-go/writer"
)

var (
	err error
)

const TSHARK_PATH = "tshark"

type Options struct {
	Source  string   `long:"source" description:"Where to get the file list from" choice:"file" choice:"queue" default:"file"`
	Input   []string `long:"input" description:"Input pcap File(s)" required:"true"`
	Output  string   `long:"output" description:"Output Directory" required:"true"`
	TempDir string   `long:"tempdir" description:"Working Directory" default:"/tmp"`
}

var opts Options

func main() {
	// parse the arguments passed in
	if _, err := flags.Parse(&opts); err != nil {
		os.Exit(1)
	}

	requests, err := FromOptions(&opts)
	if err != nil {
		log.Fatalln("Error Parsing Options:", err)
	}
	for requests.HasMore() {
		// get the next value
		requestFilePath := requests.GetValue()

		// fetch the requested file to the temporary directory
		tempFilePath, _ := requests.Fetch(requestFilePath)
		parquetFilePath := fmt.Sprintf("%s.parquet", tempFilePath)
		log.Println("Processing Packet Capture:", requestFilePath, "as", tempFilePath)
		if err := parsePacket(tempFilePath, parquetFilePath); err != nil {
			log.Fatalf("Error(s) Processing Capture: %s [%s]", requestFilePath, err)
		}

		// finished processing the capture, ensure the request is completed
		log.Println("Completed Processing Packet Capture:", requestFilePath)
		if err := requests.Complete(parquetFilePath); err != nil {
			log.Fatalln("Error Completing Request", err)
		}

		// remove the local file
		os.Remove(tempFilePath)
	}
}

func parsePacket(inputFile string, outputFile string) error {
	// ensure that the pcap file exits
	if _, err := os.Stat(inputFile); os.IsNotExist(err) {
		log.Fatalln("Invalid pcap:", inputFile)
	}

	// hard-coded for now
	cmd := exec.Command(TSHARK_PATH,
		"-r", inputFile,
		"-o", "tcp.desegment_tcp_streams:TRUE",
		"-o", "http.desegment_headers:TRUE",
		"-o", "http.desegment_body:TRUE",
		"-T", "json",
		"-e", "frame.time_epoch",
		"-e", "http.request.method",
		"-e", "http.request.uri",
		"-e", "http.x_forwarded_for",
		"-e", "http.user_agent",
		"-e", "http.referer",
		"-e", "http.file_data",
	)

	// get the stdout pipe, that's where tshark is going to dump the formatted payload,
	// don't use the -w file: https://www.wireshark.org/docs/man-pages/tshark.html
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		log.Fatalf("Error Opening Stdout Pipe: %s", err)
	}

	// get the stderr pipe, that's where were going to get any invocation errors
	stderr, err := cmd.StderrPipe()
	if err != nil {
		log.Fatalf("Error Opening Stderr Pipe: %s", err)
	}

	// start the command
	if err = cmd.Start(); err != nil {
		log.Fatalf("Error Starting Command: %s", err)
	}

	// make sure we wait at the end of *this* function for the command
	// to complete
	defer func() {
		// wait for the command to finish executing
		if err := cmd.Wait(); err != nil {
			log.Fatalf("Error Executing Command: %s", err)
		}
	}()

	// read from the stdout reader
	stdoutReader := bufio.NewReader(stdout)
	// peek at stdout and check if it has EOF, if it does, we probably had an error
	// and we should surface those
	if _, err := stdoutReader.Peek(1); err == io.EOF {
		// gross assumption that the stderr output isn't that long
		stderr, _ := io.ReadAll(stderr)
		log.Fatalf(fmt.Sprint(string(stderr)))
	}

	// pass the stdout buffer to the json decoder
	jsonDecoder := json.NewDecoder(stdoutReader)

	// packets are multivalued, so they start with a json array declaration [
	// https://github.com/wireshark/wireshark/blob/22e7ddb63789ff603641be116ee24834ca7631f9/wsutil/json_dumper.c#L282
	if _, err := jsonDecoder.Token(); err != nil {
		log.Fatalf("Error Reading First Delimiter: %s", err)
	}

	log.Println("Creating Output File", outputFile)
	fileWriter, err := local.NewLocalFileWriter(outputFile)
	if err != nil {
		log.Fatalf("Error Opening Local File Writer: %s", err)
	}

	// parquet writer
	parquetWriter, err := writer.NewParquetWriter(fileWriter, new(Parquet), 4)
	if err != nil {
		log.Fatalf("Error Creating Parquet File Writer: %s", err)
	}

	// parquet options
	parquetWriter.RowGroupSize = 128 * 1024 * 1024 // 128M
	parquetWriter.PageSize = 8 * 1024              // 8K
	parquetWriter.CompressionType = parquet.CompressionCodec_SNAPPY

	log.Println("Parsing Packets in Capture")

	// loop while there is more output to decode
	decodeCounter := 0
	decodeError := 0
	for jsonDecoder.More() {
		decodeCounter++
		log.Println("Processing Capture", decodeCounter)

		// decode into the Packet struct
		var decodedPacket Packet
		if err := jsonDecoder.Decode(&decodedPacket); err != nil {
			decodeError++
			log.Fatalf("Error Decoding Json: %s - Near Element: %d", err, decodeCounter)
		}

		// hardcoded skip of anything that isn't a POST request
		httpMethod := decodedPacket.Source.Layers.Method
		if len(httpMethod) < 1 || httpMethod[0] != http.MethodPost {
			continue
		}

		parquetColumn := Parquet{
			Filename: inputFile,
		}

		if err := parseRequest(&decodedPacket, &parquetColumn); err != nil {
			log.Fatalf("Error Parsing Parquet: %s", err)
		}

		if err := parquetWriter.Write(parquetColumn); err != nil {
			log.Fatalf("Error Writing Parquet: %s", err)
		}
	}

	log.Println("Finished Parsing Packets in Capture")

	log.Println("Stopping Parquet File Writer")
	if err = parquetWriter.WriteStop(); err != nil {
		log.Fatalln("Error Stopping Writer", err)
	}

	// we expect a well formed json document with an ending delimiter too
	// https://github.com/wireshark/wireshark/blob/22e7ddb63789ff603641be116ee24834ca7631f9/wsutil/json_dumper.c#L296
	if _, err := jsonDecoder.Token(); err != nil {
		log.Fatalln("Error Reading Final Delimiter:", err)
	}

	if decodeError > 0 {
		log.Printf(fmt.Sprint("Error(s) Logged During Decode:", decodeError))
	}

	return nil
}
