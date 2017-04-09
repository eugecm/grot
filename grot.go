package main

import (
	"bufio"
	"flag"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"os"
	"path/filepath"
	"time"
)

const lineBufferSize = 10000
const maxLogs = 9

var every = flag.Duration("every", 1*time.Minute, "How often to rotate")
var outputFile = flag.String("to", "output.log", "Output file to write to")

func main() {
	log.SetPrefix("grot")

	flag.Parse()
	lines := setupInputChannel(os.Stdin)
	handleOutput(*outputFile, *every, lines)
}

func handleOutput(filename string, every time.Duration, input <-chan string) {
	fileH, err := rotate(filename, maxLogs)
	if err != nil {
		log.Panicf("could not perform initial rotation: %v", err)
	}

	writer := bufio.NewWriter(fileH)
	timer := time.NewTicker(every)
	for {
		select {
		case line, ok := <-input:
			if !ok {
				err = fileH.Close()
				if err != nil {
					log.Panicf("Could not close file: %v", err)
				}
				return
			}
			_, err := writer.Write([]byte(line))
			if err != nil {
				log.Panicf("An error occured while writing to %v: %v", filename, err)
			}
		case <-timer.C:
			err := writer.Flush()
			if err != nil {
				log.Panicf("Could not flush contents to file on rotation: %v", err)
			}

			err = fileH.Close()
			if err != nil {
				log.Panicf("Could not close file on rotation: %v", err)
			}

			fileH, err = rotate(filename, maxLogs)
			if err != nil {
				log.Panicf("error while performing rotation: %v", err)
			}
			writer.Reset(fileH)
		}
	}
}

func setupInputChannel(input io.Reader) <-chan string {
	reader := bufio.NewReader(input)
	lines := make(chan string, lineBufferSize)
	go func() {
		for {
			line, err := reader.ReadString('\n')
			if err != nil {
				break
			}
			lines <- line
		}
		close(lines)
	}()

	return lines
}

func rotate(filename string, keep int) (*os.File, error) {

	// make sure filename is clean before we work with it
	filename = filepath.Clean(filename)

	// read directory
	workingDir := filepath.Dir(filename)
	files, err := ioutil.ReadDir(workingDir)
	if err != nil {
		return nil, fmt.Errorf("could not open directory %v: %v", workingDir, err)
	}

	// load files in a "set"
	existingFiles := make(map[string]bool)
	for _, file := range files {
		existingFiles[file.Name()] = true
	}

	// perform rotation
	for logNumber := keep; logNumber > 0; logNumber-- {

		logName := fmt.Sprintf("%v.%v", filename, logNumber)
		logAbsPath := filepath.Join(workingDir, logName)

		// log doesn't exist yet. skip
		if !existingFiles[logName] {
			continue
		}

		// log is last to keep, delete
		if logNumber == keep {
			err := os.Remove(logAbsPath)
			if err != nil {
				return nil, fmt.Errorf("could not delete last log file: %v", err)
			}
			continue
		}

		newLogName := filepath.Join(workingDir, fmt.Sprintf("%v.%v", filename, logNumber+1))
		err := os.Rename(logAbsPath, newLogName)
		if err != nil {
			return nil, fmt.Errorf("could not rename %v into %v", logAbsPath, newLogName)
		}
	}

	// If main file exists, rotate it
	newLogName := filepath.Join(workingDir, fmt.Sprintf("%v.1", filename))
	err = os.Rename(filename, newLogName)
	if err != nil && !os.IsNotExist(err) {
		return nil, fmt.Errorf("could not rename %v into %v: %v", filename, newLogName, err)

	}

	fileH, err := os.Create(filename)
	if err != nil {
		return nil, fmt.Errorf("could not create base file %v: %v", filename, err)
	}

	return fileH, nil
}
