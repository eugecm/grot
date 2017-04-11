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

const defaultDuration = 1 * time.Minute
const defaultOutputFile = "output.log"
const defaultMaxLogs = 9

var every = flag.Duration("every", defaultDuration, "How often to rotate")
var outputFile = flag.String("to", defaultOutputFile, "Output file to write to")
var maxLogs = flag.Int("max", defaultMaxLogs, "How many logs to keep")

func main() {
	log.SetPrefix("grot")
	flag.Parse()

	lines := setupInputChannel(os.Stdin)
	handleOutput(*outputFile, *every, lines)
}

func mustCloseFile(fileH *os.File, panicMsg string) {
	err := fileH.Close()
	if err != nil {
		log.Panicf("%s: %v", panicMsg, err)
	}
}

func handleOutput(filename string, every time.Duration, input <-chan string) {
	fileH, err := rotate(filename, *maxLogs)
	if err != nil {
		log.Panicf("could not perform initial rotation: %v", err)
	}

	timer := time.NewTicker(every)
	for {
		select {
		case line, ok := <-input:
			if !ok {
				mustCloseFile(fileH, fmt.Sprintf("Could not close file"))
				return
			}

			_, err := fileH.WriteString(line)
			if err != nil {
				log.Panicf("An error occured while writing to %v: %v", filename, err)
			}
		case <-timer.C:

			mustCloseFile(fileH, fmt.Sprintf("Could not close file on rotation: %v", err))

			fileH, err = rotate(filename, *maxLogs)
			if err != nil {
				log.Panicf("error while performing rotation: %v", err)
			}
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

func listDirectory(dirPath string) (map[string]*os.FileInfo, error) {

	workingDir := filepath.Clean(dirPath)
	files, err := ioutil.ReadDir(workingDir)
	if err != nil {
		return nil, fmt.Errorf("could not open directory %v: %v", workingDir, err)
	}

	// load files in a "set"
	fileSet := make(map[string]*os.FileInfo)
	for i := range files {
		fileSet[files[i].Name()] = &files[i]
	}

	return fileSet, nil
}

func rotate(filename string, keep int) (*os.File, error) {

	// make sure filename is clean before we work with it
	filename = filepath.Clean(filename)

	workingDir := filepath.Dir(filename)
	existingFiles, err := listDirectory(workingDir)
	if err != nil {
		log.Panicf("Could not list list files: %v", err)
	}

	// perform rotation
	for logNumber := keep; logNumber > 0; logNumber-- {

		logName := fmt.Sprintf("%v.%v", filename, logNumber)
		logAbsPath := filepath.Join(workingDir, logName)

		// log doesn't exist yet. skip
		if _, ok := existingFiles[logName]; !ok {
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
