package workflow

type Phase struct {
	Tasks []Task
}

type Task struct {
	Name  string
	Tasks []Task
}

func NewPhase() *Phase {
	return &Phase{
		Tasks: []Task{},
	}
}

func (p *Phase) AppendTask(t Task) {
	p.Tasks = append(p.Tasks, t)
}

func (p *Phase) Run() error {
	//TODO Get the data required for the current task and execute the task
	return nil
}
