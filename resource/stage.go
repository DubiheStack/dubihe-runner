package resource

import (
	"fmt"

	"gopkg.in/yaml.v3"
)

// Stage 表示流水线阶段
type Stage struct {
	Key       string  `yaml:"key,omitempty"`
	Name      string  `yaml:"name,omitempty"`
	Condition string  `yaml:"condition,omitempty"` // 阶段执行条件
	Jobs      JobList `yaml:"jobs,omitempty"`
}

// StageList 阶段列表
type StageList []*Stage

// Index 根据key查找阶段
func (list StageList) Index(key string) (*Stage, bool) {
	for _, stg := range list {
		if stg.Key == key {
			return stg, true
		}
	}
	return nil, false
}

// UnmarshalYAML 自定义YAML反序列化
func (l *StageList) UnmarshalYAML(node *yaml.Node) error {
	if node.Kind != yaml.MappingNode {
		return fmt.Errorf("unexpected non-mapping stage node")
	}

	var i int
	for i < len(node.Content) {
		stgKey := node.Content[i].Value
		stg := &Stage{}
		err := node.Content[i+1].Decode(stg)
		if err != nil {
			return err
		}
		stg.Key = stgKey
		*l = append(*l, stg)
		i += 2
	}
	return nil
}
