package main

import "sync"

type runner struct {
	tasks []func()
}

func (r *runner) run() {
	wg := &sync.WaitGroup{}
	for _, task := range r.tasks {
		wg.Go(func() {
			task()
		})
	}
	wg.Wait()
}

func newRunner(tasks []func()) *runner {
	return &runner{
		tasks: tasks,
	}
}
