//
// Copyright (C) 2020 IBM Corporation.
//
// Authors:
// Frederico Araujo <frederico.araujo@ibm.com>
// Teryl Taylor <terylt@ibm.com>
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.
//
package encoders

import (
	cqueue "github.com/enriquebris/goconcurrentqueue"
	cmap "github.com/orcaman/concurrent-map"
	"github.com/sysflow-telemetry/sf-processor/core/exporter/commons"
	"github.com/sysflow-telemetry/sf-processor/core/policyengine/engine"
)

// Occurrence object for IBM Findings API.
type Occurrence struct {
	ID         string
	ShortDescr string
	LongDescr  string
	Severity   Severity
	Certainty  Certainty
	ResType    string
	ResName    string
	AlertQuery string
	NoteID     string
}

// OccurrenceEncoder is an encoder for IBM Findings' occurrences.
type OccurrenceEncoder struct {
	config      commons.Config
	exportQueue *cqueue.FIFO
}

func NewOccurrenceEncoder(conf commons.Config) Encoder {
	queue := cqueue.NewFIFO()
	queue.Enqueue(cmap.New())
	return &OccurrenceEncoder{config: conf, exportQueue: queue}
}

// Register registers the encoder to the codecs cache.
func (oe *OccurrenceEncoder) Register(codecs map[commons.Format]EncoderFactory) {
	codecs[commons.OccurrenceFormat] = NewOccurrenceEncoder
}

// Encodes a telemetry record into an occurrence representation.
func (oe *OccurrenceEncoder) Encode(r *engine.Record) (data commons.EncodedData, err error) {
	return
}

// addEvent adds a record to export queue.
func (oe *OccurrenceEncoder) addEvent(r *engine.Record) {
	// r.Container[]
	// s.exportQueue.Get(0)[]
}