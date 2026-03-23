package resource

import (
	"errors"

	"gopkg.in/yaml.v3"
)

var (
	ErrSourceNotFound   = errors.New("no source is found")
	ErrStageNotFound    = errors.New("stage not found")
	ErrJobNotFound      = errors.New("job not found")
	ErrInvalidPipeline  = errors.New("invalid pipeline configuration")
)

// ParseYaml 解析云效流水线YAML配置
func ParseYaml(data []byte) (*Pipeline, error) {
	pipeline := &Pipeline{}
	err := yaml.Unmarshal(data, pipeline)
	if err != nil {
		return nil, err
	}
	return pipeline, Lint(pipeline)
}

// ParseYamlString 从字符串解析流水线配置
func ParseYamlString(config string) (*Pipeline, error) {
	return ParseYaml([]byte(config))
}

// Lint 校验流水线配置
func Lint(pipeline *Pipeline) error {
	if pipeline == nil {
		return ErrInvalidPipeline
	}

	// 校验阶段
	stageKeys := make(map[string]struct{})
	for _, stage := range pipeline.Stages {
		if stage == nil {
			return errors.New("lint: detected nil stage")
		}
		if stage.Key == "" {
			return errors.New("lint: invalid or missing stage key")
		}
		if len(stage.Key) > 100 || len(stage.Name) > 100 {
			return errors.New("lint: stage key or name cannot exceed 100 characters")
		}
		if _, ok := stageKeys[stage.Key]; ok {
			return errors.New("lint: duplicate stage key")
		}
		stageKeys[stage.Key] = struct{}{}

		// 校验任务
		jobKeys := make(map[string]struct{})
		for _, job := range stage.Jobs {
			if job == nil {
				return errors.New("lint: detected nil job")
			}
			if job.Key == "" {
				return errors.New("lint: invalid or missing job key")
			}
			if _, ok := jobKeys[job.Key]; ok {
				return errors.New("lint: duplicate job key in stage")
			}
			jobKeys[job.Key] = struct{}{}

			// 校验步骤
			stepKeys := make(map[string]struct{})
			for _, step := range job.Steps {
				if step == nil {
					return errors.New("lint: detected nil step")
				}
				if step.Key == "" {
					return errors.New("lint: invalid or missing step key")
				}
				if _, ok := stepKeys[step.Key]; ok {
					return errors.New("lint: duplicate step key in job")
				}
				stepKeys[step.Key] = struct{}{}
			}
		}
	}

	// 校验代码源
	srcKeys := make(map[string]struct{})
	for _, src := range pipeline.Sources {
		if src == nil {
			return errors.New("lint: detected nil source")
		}
		if src.Key == "" {
			return errors.New("lint: invalid or missing source key")
		}
		if len(src.Key) > 100 || len(src.Name) > 100 {
			return errors.New("lint: source key or name cannot exceed 100 characters")
		}
		if _, ok := srcKeys[src.Key]; ok {
			return errors.New("lint: duplicate source key")
		}
		srcKeys[src.Key] = struct{}{}
	}

	// 校验默认工作空间
	if pipeline.DefaultWorkspace != "" {
		if _, ok := srcKeys[pipeline.DefaultWorkspace]; !ok {
			return errors.New("lint: default workspace source not found")
		}
	}

	return nil
}

// LookupStage 查找阶段
func (p *Pipeline) LookupStage(key string) (*Stage, error) {
	stage, ok := p.Stages.Index(key)
	if !ok {
		return nil, ErrStageNotFound
	}
	return stage, nil
}

// LookupJob 查找任务
func (p *Pipeline) LookupJob(stageKey, jobKey string) (*Job, error) {
	stage, err := p.LookupStage(stageKey)
	if err != nil {
		return nil, err
	}
	job, ok := stage.Jobs.Index(jobKey)
	if !ok {
		return nil, ErrJobNotFound
	}
	return job, nil
}
