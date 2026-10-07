package pipeline

import (
	"custom-rules/internal/types"
	"fmt"
	"time"
)

// Stage is one discrete step of the asset build process.
type Stage interface {
	Name() string
	Execute(ctx *types.BuildContext) error
}

// Pipeline runs an ordered list of Stages against a shared  BuildContext.
type Pipeline struct {
	stages []Stage
}

// NewPipeline creates an empty  Pipeline.
func NewPipeline() *Pipeline {
	return &Pipeline{}
}

// AddStage append a Stage to the pipeline's run order.
func (p *Pipeline) AddStage(s Stage) {
	p.stages = append(p.stages, s)
}

// Run executes every stage in order, stopping at the  first error.
func (p *Pipeline) Run(ctx *types.BuildContext) error {
	for _, s := range p.stages {
		start := time.Now()
		fmt.Printf("[Pipeline] Excuting stage: %s... \n", s.Name())
		if err := s.Execute(ctx); err != nil {
			return fmt.Errorf("stage '%s' failed: %w", s.Name(), err)
		}

		fmt.Printf("[Pipeline] Stage '%s' completed in %v\n", s.Name(), time.Since(start))
	}
	return nil
}