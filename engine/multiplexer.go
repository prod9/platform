package engine

import (
	"sync"
)

// multiplexer is for embedding into another struct to provide a simple way to multiplex
// work process and collect results.
type multiplexer[TIn any, TOut any] struct {
	sync.Mutex // prevent simultaneous write to results

	inputs  []TIn
	outputs []TOut
}

func (m *multiplexer[TIn, TOut]) Reset(inputs []TIn) {
	m.Lock()
	defer m.Unlock()

	m.inputs = inputs
	m.outputs = nil
}

func (m *multiplexer[TIn, TOut]) Start(work func(idx int, input TIn) TOut) []TOut {
	wg := sync.WaitGroup{}
	for idx, job := range m.inputs {
		wg.Add(1)
		go func(idx int, job TIn) {
			defer wg.Done()
			result := work(idx, job)
			m.setOutput(idx, result)
		}(idx, job)
	}
	wg.Wait()

	return m.outputs
}

func (m *multiplexer[TIn, TOut]) setOutput(idx int, result TOut) {
	m.Lock()
	defer m.Unlock()

	if m.outputs == nil {
		m.outputs = make([]TOut, len(m.inputs))
	}

	m.outputs[idx] = result
}
