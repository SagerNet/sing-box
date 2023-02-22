package libbox

import (
	"bufio"
	"log"
	"os"
)

type StandardOutput interface {
	WriteOutput(message string)
	WriteErrorOutput(message string)
}

func SetOutput(output StandardOutput) {
	log.SetOutput(logWriter{output})
	pipeIn, pipeOut, err := os.Pipe()
	if err != nil {
		panic(err)
	}
	os.Stdout = os.NewFile(pipeOut.Fd(), "stdout")
	go lineLog(pipeIn, output.WriteOutput)

	pipeIn, pipeOut, err = os.Pipe()
	if err != nil {
		panic(err)
	}
	os.Stderr = os.NewFile(pipeOut.Fd(), "srderr")
	go lineLog(pipeIn, output.WriteErrorOutput)
}

type logWriter struct {
	output StandardOutput
}

func (w logWriter) Write(p []byte) (n int, err error) {
	w.output.WriteOutput(string(p))
	return len(p), nil
}

func lineLog(f *os.File, output func(string)) {
	const logSize = 1024 // matches android/log.h.
	r := bufio.NewReaderSize(f, logSize)
	for {
		line, _, err := r.ReadLine()
		str := string(line)
		if err != nil {
			str += " " + err.Error()
		}
		output(str)
		if err != nil {
			break
		}
	}
}
