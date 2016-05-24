/*
Copyright 2016 Medcl (m AT medcl.net)

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

   http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package main

import (
	"sync"
	"gopkg.in/cheggaaa/pb.v1"
	log "github.com/cihub/seelog"
	"os"
	"bufio"
	"encoding/json"
	"io"
)

func checkFileIsExist(filename string) (bool) {
	var exist = true;
	if _, err := os.Stat(filename); os.IsNotExist(err) {
		exist = false;
	}
	return exist;
}

// strip '\n' or read until EOF, return error if read error
func readLine(reader io.Reader) (line []byte, err error) {
	line = make([]byte, 0, 100)
	for {
		b := make([]byte, 1)
		n, er := reader.Read(b)
		if n > 0 {
			c := b[0]
			if c == '\n' { // end of line
				break
			}
			line = append(line, c)
		}
		if er != nil {
			err = er
			return
		}
	}
	return
}

func (m *Migrator) NewFileReadWorker(pb *pb.ProgressBar, wg *sync.WaitGroup)  {
	log.Debug("start reading file")
	f, err := os.Open(m.Config.DumpInputFile)
	if err != nil {
		log.Error(err)
		return
	}
	scanner := bufio.NewScanner(f)
	scanner.Split(bufio.ScanLines)
	lineCount := 0
	for scanner.Scan() {
		lineCount++
		js := map[string]interface{}{}
		line := scanner.Text()
		//log.Trace("reading file,",lineCount,",", line)
		err = json.Unmarshal([]byte(line), &js)
		if(err!=nil){
			log.Error(err)
			continue
		}
		m.DocChan <- js
		pb.Increment()
	}

	defer f.Close()
	log.Debug("end reading file")
	close(m.DocChan)
	wg.Done()
}

func (c *Migrator) NewFileDumpWorker(pb *pb.ProgressBar, wg *sync.WaitGroup) {
	var f *os.File
	var err1   error;

	if checkFileIsExist(c.Config.DumpOutFile) {
		f, err1 = os.OpenFile(c.Config.DumpOutFile, os.O_APPEND|os.O_WRONLY, os.ModeAppend)
		if(err1!=nil){
			log.Error(err1)
			return
		}

	}else {
		f, err1 = os.Create(c.Config.DumpOutFile)
		if(err1!=nil){
			log.Error(err1)
			return
		}
	}

	w := bufio.NewWriter(f)

	READ_DOCS:
	for {
		docI, open := <-c.DocChan

		// this check is in case the document is an error with scroll stuff
		if status, ok := docI["status"]; ok {
			if status.(int) == 404 {
				log.Error("error: ", docI["response"])
				continue
			}
		}

		// sanity check
		for _, key := range []string{"_index", "_type", "_source", "_id"} {
			if _, ok := docI[key]; !ok {
				//json,_:=json.Marshal(docI)
				//log.Errorf("failed parsing document: %v", string(json))
				break READ_DOCS
			}
		}

		jsr,err:=json.Marshal(docI)
		log.Debug(string(jsr))
		if(err!=nil){
			log.Error(err)
		}
		n,err:=w.WriteString(string(jsr))
		if(err!=nil){
			log.Error(n,err)
		}
		w.WriteString("\n")
		pb.Increment()

		// if channel is closed flush and gtfo
		if !open {
			goto WORKER_DONE
		}
	}

	WORKER_DONE:
	w.Flush()
	f.Close()

	wg.Done()
	log.Debug("file dump finished")
}


