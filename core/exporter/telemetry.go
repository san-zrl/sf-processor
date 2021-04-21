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
package exporter

import (
	"encoding/json"
	"fmt"
	"reflect"
	"strings"

	"github.com/sysflow-telemetry/sf-apis/go/sfgo"
	"github.com/sysflow-telemetry/sf-processor/core/policyengine/engine"
)

// SysFlow record components
const (
	proc      = "proc"
	pproc     = "pproc"
	net       = "net"
	file      = "file"
	flow      = "flow"
	container = "container"
	node      = "node"
)

// TelemetryRecord type
type TelemetryRecord struct {
	Version     string `json:"version,omitempty"`
	*FlatRecord `json:",omitempty"`
	*DataRecord `json:",omitempty"`
	Hashes      *engine.HashSet `json:"hashes,omitempty"`
	Policies    []Policy        `json:"policies,omitempty"`
	AsECS       bool
}

// FlatRecord type
type FlatRecord struct {
	Data map[string]interface{} `json:"record"`
}

// DataRecord type (warning: make sure field names have only first letter capitalized)
type DataRecord struct {
	Type       string   `json:"type"`
	Opflags    []string `json:"opflags"`
	Ret        int64    `json:"ret"`
	Ts         int64    `json:"ts"`
	Endts      int64    `json:"endts,omitempty"`
	Schema     int64    `json:"schema,omitempty"`
	*ProcData  `json:",omitempty"`
	*PprocData `json:",omitempty"`
	*NetData   `json:",omitempty"`
	*FileData  `json:",omitempty"`
	*FlowData  `json:",omitempty"`
	*ContData  `json:",omitempty"`
	*NodeData  `json:",omitempty"`
}

// ProcData type
type ProcData struct {
	Proc map[string]interface{} `json:"proc"`
}

// PprocData type
type PprocData struct {
	Pproc map[string]interface{} `json:"pproc"`
}

// NetData type
type NetData struct {
	Net map[string]interface{} `json:"net"`
}

// FileData type
type FileData struct {
	File map[string]interface{} `json:"file"`
}

// FlowData type
type FlowData struct {
	Flow map[string]interface{} `json:"flow"`
}

// ContData type
type ContData struct {
	Container map[string]interface{} `json:"container"`
}

// NodeData type
type NodeData struct {
	Node map[string]interface{} `json:"node"`
}

// CreateTelemetryRecords creates offense instances based on a list of records
func CreateTelemetryRecords(occs []*engine.Record, config Config) []Event {
	var recs = make([]Event, 0)
	for _, o := range occs {
		recs = append(recs, extractTelemetryRecord(o, config))
	}
	return recs
}

// ToJSONStr returns a JSON string representation of an observation
func (s TelemetryRecord) ToJSONStr() string {
	return string(s.ToJSON())
}

// ToJSON returns a JSON bytearray representation of an observation
func (s TelemetryRecord) ToJSON() []byte {
	var o []byte
	if s.AsECS {
		o, _ = json.Marshal(ToECS(s))
	} else {
		o, _ = json.Marshal(s)
	}
	return o
}

func (s TelemetryRecord) ID() string {
	var h = make([]interface{}, 0)
	if s.FlatRecord != nil {
		data := s.FlatRecord.Data
		t := data[engine.SF_TYPE].(string)
		h = append(h, data[engine.SF_NODE_ID],
			      data[engine.SF_CONTAINER_ID],
			      data[engine.SF_TS],
			      data[engine.SF_PROC_TID],
			      data[engine.SF_PROC_CREATETS],
			      t)
		switch t {
		case sfgo.TyFFStr, sfgo.TyFEStr:
			h = append(h, data[engine.SF_FILE_OID])
		case sfgo.TyNFStr:
			h = append(h, data[engine.SF_NET_SIP],
			              data[engine.SF_NET_SPORT],
				      data[engine.SF_NET_DIP],
				      data[engine.SF_NET_DPORT],
				      data[engine.SF_NET_PROTO])
		}
	} else {
		data := s.DataRecord
		h = append(h, data.NodeData.Node[LastPart(engine.SF_NODE_ID)])
		if data.ContData != nil {
			if v, ok := data.ContData.Container[LastPart(engine.SF_CONTAINER_ID)]; ok {
				h = append(h, v)
			}
		}
		h = append(h, data.Ts,
			      data.ProcData.Proc[LastPart(engine.SF_PROC_TID)],
			      data.ProcData.Proc[LastPart(engine.SF_PROC_CREATETS)],
			      data.Type)
		switch data.Type {
		case sfgo.TyFFStr, sfgo.TyFEStr:
			h = append(h, data.FileData.File[LastPart(engine.SF_FILE_OID)])
		case sfgo.TyNFStr:
			net := data.NetData.Net
			h = append(h, net[LastPart(engine.SF_NET_SIP)],
			              net[LastPart(engine.SF_NET_SPORT)],
				      net[LastPart(engine.SF_NET_DIP)],
				      net[LastPart(engine.SF_NET_DPORT)],
				      net[LastPart(engine.SF_NET_PROTO)])
		}
	}
	return GetHashStr(h)
}

func extractTelemetryRecord(rec *engine.Record, config Config) TelemetryRecord {
	r := TelemetryRecord{}
	r.AsECS = (config.Format == ECSFormat)
	r.Version = config.JSONSchemaVersion
	if config.Flat {
		r.FlatRecord = new(FlatRecord)
		r.FlatRecord.Data = make(map[string]interface{})
		for _, k := range engine.Fields {
			r.Data[k] = engine.Mapper.Mappers[k](rec)
		}
	} else {
		r.DataRecord = new(DataRecord)
		pprocID := engine.Mapper.MapInt(engine.SF_PPROC_PID)(rec)
		pprocExists := !reflect.ValueOf(pprocID).IsZero()
		ct := engine.Mapper.MapStr(engine.SF_CONTAINER_ID)(rec)
		ctExists := !reflect.ValueOf(ct).IsZero()
		for _, k := range engine.Fields {
			kc := strings.Split(k, ".")
			value := extractValue(k, engine.Mapper.Mappers[k](rec))
			if len(kc) == 2 {
				switch value.(type) {
				case string:
					reflect.ValueOf(r.DataRecord).Elem().FieldByName(strings.Title(kc[1])).SetString(value.(string))
				case int64:
					reflect.ValueOf(r.DataRecord).Elem().FieldByName(strings.Title(kc[1])).SetInt(value.(int64))
				case []string:
					reflect.ValueOf(r.DataRecord).Elem().FieldByName(strings.Title(kc[1])).Set(reflect.ValueOf(value))
				}
			} else if len(kc) == 3 {
				switch kc[1] {
				case proc:
					if r.ProcData == nil {
						r.ProcData = new(ProcData)
						r.ProcData.Proc = make(map[string]interface{})
					}
					r.Proc[kc[2]] = value
				case pproc:
					if pprocExists {
						if r.PprocData == nil {
							r.PprocData = new(PprocData)
							r.PprocData.Pproc = make(map[string]interface{})
						}
						r.Pproc[kc[2]] = value
					}
				case net:
					if r.Type == sfgo.TyNFStr {
						if r.NetData == nil {
							r.NetData = new(NetData)
							r.NetData.Net = make(map[string]interface{})
						}
						r.Net[kc[2]] = value
					}
				case file:
					if r.Type == sfgo.TyFFStr || r.Type == sfgo.TyFEStr {
						if r.FileData == nil {
							r.FileData = new(FileData)
							r.FileData.File = make(map[string]interface{})
						}
						r.File[kc[2]] = value
					}
				case flow:
					if r.Type == sfgo.TyFFStr || r.Type == sfgo.TyNFStr {
						if r.FlowData == nil {
							r.FlowData = new(FlowData)
							r.FlowData.Flow = make(map[string]interface{})
						}
						r.Flow[kc[2]] = value
					}
				case container:
					if ctExists {
						if r.ContData == nil {
							r.ContData = new(ContData)
							r.ContData.Container = make(map[string]interface{})
						}
						r.Container[kc[2]] = value
					}
				case node:
					if r.NodeData == nil {
						r.NodeData = new(NodeData)
						r.NodeData.Node = make(map[string]interface{})
					}
					r.Node[kc[2]] = value
				}
			}
		}
	}
	hashset := rec.Ctx.GetHashes()
	if !reflect.ValueOf(hashset.MD5).IsZero() {
		r.Hashes = &hashset
	}
	r.Policies = extractPolicySet(rec.Ctx.GetRules())
	return r
}

func extractPolicySet(rules []engine.Rule) []Policy {
	var pols = make([]Policy, 0)
	for _, r := range rules {
		p := Policy{
			ID:       r.Name,
			Desc:     r.Desc,
			Priority: int(r.Priority),
			Tags:     extracTags(r.Tags),
		}
		pols = append(pols, p)
	}
	return pols
}

func extracTags(tags []engine.EnrichmentTag) []string {
	s := make([]string, 0)
	for _, v := range tags {
		switch v.(type) {
		case []string:
			s = append(s, v.([]string)...)
			break
		default:
			s = append(s, string(fmt.Sprintf("%v", v)))
			break
		}
	}
	return s
}

func extractValue(k string, v interface{}) interface{} {
	switch v.(type) {
	case string:
		if array(k) {
			return strings.Split(v.(string), engine.LISTSEP)
		}
		return v
	default:
		return v
	}
}

func array(k string) bool {
	return k == engine.SF_OPFLAGS || k == engine.SF_PROC_APID || k == engine.SF_PROC_ANAME ||
		k == engine.SF_PROC_AEXE || k == engine.SF_PROC_ACMDLINE || k == engine.SF_FILE_OPENFLAGS ||
		k == engine.SF_NET_IP || k == engine.SF_NET_PORT
}
