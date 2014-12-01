package cinterop

import (
	"io"
	"log"
)

// this reads in a loop from socketRead putting batchSize bytes of work to copyTo until
// the socketRead is empty. Will always block until a full workSize of units have been copied
func readBuffer(copyTo chan<- []byte, socketRead io.Reader, batchSize int, workSize int) {
	defer close(copyTo)
	for {
		batch := make([]byte, batchSize)
		size, err := socketRead.Read(batch)
		if err == nil && size%workSize != 0 {
			var lsize int
			lsize, err = io.ReadFull(socketRead, batch[size:size+workSize-(size%workSize)])
			size += lsize
		}
		if size > 0 {
			copyTo <- batch[:size]
		}
		if err != nil {
			if err != io.EOF {
				log.Print("Error encountered in readBuffer:", err)
			}
			return
		}
	}
}

// this simply copies data from the chan to the socketWrite writer
func writeBuffer(copyFrom <-chan []byte, socketWrite io.Writer) {
	for buf := range copyFrom {
		if len(buf) > 0 {
			_, err := socketWrite.Write(buf)
			if err != nil {
				log.Print("Error encountered in writeBuffer:", err)
				return
			}
		}
	}
}

// this function takes data from socketRead and calls processBatch on a batch of it at a time
// then the resulting bytes are written to wocketWrite as fast as possible
func ProcessBufferedData(socketRead io.Reader, socketWrite io.Writer,
	makeProcessBatch func() (func(input []byte) []byte,
		func(lastInput []byte, lastOutput []byte)),
	batchSize, workItemSize int) {
	readChan := make(chan []byte, 2)
	writeChan := make(chan []byte, 1+batchSize/workItemSize)
	go readBuffer(readChan, socketRead, batchSize, workItemSize)
	go writeBuffer(writeChan, socketWrite)
	pastInit := false
	defer func() { // this is if makeProcessBatch() fails
		if !pastInit {
			if r := recover(); r != nil {
				log.Print("Error in makeProcessBatch ", r)
			}
		}
		close(writeChan)
	}()
	processBatch, prefetchBatch := makeProcessBatch()
	pastInit = true
	for buf := range readChan {
		result := processBatch(buf)
		writeChan <- result
		prefetchBatch(buf, result)
	}
}
