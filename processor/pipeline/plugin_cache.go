package pipeline

import (
	"errors"
	"github.com/spf13/viper"
	//_ "github.ibm.com/sysflow/sf-processor/plugins/flattener"
	//_ "github.ibm.com/sysflow/sf-processor/plugins/flattener/types"
	//_ "github.ibm.com/sysflow/sf-processor/plugins/processor"
	//"github.com/sysflow-telemetry/sf-apis/goapis/go/handlers"
	"github.com/sysflow-telemetry/sf-apis/go/handlers"
	sp "github.com/sysflow-telemetry/sf-apis/go/processors"
	"github.ibm.com/sysflow/sf-processor/common/logger"
	hdl "github.ibm.com/sysflow/sf-processor/plugins/flattener"
	proc "github.ibm.com/sysflow/sf-processor/plugins/processor"
	pol "github.ibm.com/sysflow/sf-processor/plugins/sfpe"
	"plugin"
	"strings"
)

type PluginCache struct {
	chanMap     map[string]interface{}
	pluginMap   map[string]*plugin.Plugin
	procFuncMap map[string]interface{}
	hdlFuncMap  map[string]interface{}
	chanFuncMap map[string]interface{}
	config      *viper.Viper
}

func NewPluginCache() *PluginCache {
	plug := &PluginCache{config: viper.New(), chanMap: make(map[string]interface{}), pluginMap: make(map[string]*plugin.Plugin)}
	plug.procFuncMap = map[string]interface{}{"SysFlowProc": proc.NewSysFlowProc,
		"PolicyEngine": pol.NewPolicyEngine}
	plug.hdlFuncMap = map[string]interface{}{"Flattener": hdl.NewFlattener}
	plug.chanFuncMap = map[string]interface{}{"SysFlowChan": proc.NewSysFlowChan,
		"FlattenerChan": hdl.NewFlattenerChan}
	plug.config.SetConfigName("pipeline")
	plug.config.SetConfigType("json")
	plug.config.AddConfigPath("./")
	return plug
}

func (p PluginCache) GetConfig() (*PluginConfig, error) {
	conf := new(PluginConfig)
	err := p.config.ReadInConfig()

	if err != nil {
		return nil, err
	}

	err = p.config.Unmarshal(conf)
	if err != nil {
		return nil, err
	}
	return conf, nil
}

func (p PluginCache) GetPlugin(mod string) (*plugin.Plugin, error) {
	var plug *plugin.Plugin
	var err error
	if val, ok := p.pluginMap[mod]; ok {
		plug = val
	} else {
		plug, err = plugin.Open(mod)
		if err != nil {
			return nil, err
		}
		p.pluginMap[mod] = plug
	}
	return plug, nil
}

func (p PluginCache) GetHandler(mod string, name string) (handlers.SFHandler, error) {

	var hdl handlers.SFHandler
	if val, ok := p.hdlFuncMap[name]; ok {
		funct := val.(func() handlers.SFHandler)
		hdl = funct()
	} else {

		fName := "New" + name
		plug, err := p.GetPlugin(mod)
		if err != nil {
			return nil, err
		}
		symFlattener, err := plug.Lookup(fName)
		if err != nil {
			return nil, err
		}
		funct, ok := symFlattener.(func() handlers.SFHandler)
		if !ok {
			return nil, errors.New("Unexpected type from module symbol for handler function: " + fName)
		}
		hdl = funct()
	}
	return hdl, nil
}

func (p PluginCache) GetChan(mod string, ch string, size int) (interface{}, error) {
	fields := strings.Fields(ch)
	if len(fields) != 2 {
		return nil, errors.New("Channel must be of the form <identifier> <type>")
	}

	if val, ok := p.chanMap[fields[0]]; ok {
		logger.Trace.Println("Found existing channel ", fields[0])
		return val, nil
	}
	var c interface{}
	if val, ok := p.chanFuncMap[fields[1]]; ok {
		funct := val.(func(int) interface{})
		c = funct(size)
	} else {
		plug, err := p.GetPlugin(mod)
		if err != nil {
			return nil, err
		}

		fName := "New" + fields[1]
		symChan, err := plug.Lookup(fName)
		if err != nil {
			return nil, err
		}
		funct, ok := symChan.(func(int) interface{})
		if !ok {
			return nil, errors.New("unexpected type from module symbol for channel function: " + fName)
		}
		c = funct(size)
	}
	p.chanMap[fields[0]] = c
	//c := symChan(size)
	return c, nil
}

func (p PluginCache) GetProcessor(mod string, name string, hdl handlers.SFHandler, hdlr bool) (sp.SFProcessor, error) {
	var prc sp.SFProcessor
	if val, ok := p.procFuncMap[name]; ok {
		logger.Trace.Println("Found processor in function map", name)
		if hdlr {
			funct := val.(func(handlers.SFHandler) sp.SFProcessor)
			prc = funct(hdl)
		} else {
			funct := val.(func() sp.SFProcessor)
			prc = funct()
		}
	} else {
		fName := "New" + name
		plug, err := p.GetPlugin(mod)
		if err != nil {
			return nil, err
		}
		logger.Trace.Println("FName: ", fName)
		logger.Trace.Println("plug: ", plug)
		symProcessor, err := plug.Lookup(fName)
		if err != nil {
			return nil, err
		}
		if hdlr {
			funct, ok := symProcessor.(func(handlers.SFHandler) sp.SFProcessor)
			if !ok {
				return nil, errors.New("unexpected type from module symbol for processor: " + fName)
			}
			prc = funct(hdl)
		} else {
			funct, ok := symProcessor.(func() sp.SFProcessor)
			if !ok {
				return nil, errors.New("unexpected type from module symbol for processor: " + fName)
			}
			prc = funct()
		}
	}
	return prc, nil
}