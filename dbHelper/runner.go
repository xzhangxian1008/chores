package main

type runner struct {
	tasks []func()
}

func (r *runner) run() {
	for _, task := range r.tasks {
		task()
	}
}

func newRunner(tasks []func()) *runner {
	return &runner{
		tasks: tasks,
	}
}
