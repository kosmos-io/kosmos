package workflow

import "k8s.io/klog/v2"

type Phase struct {
	Tasks              []Task
	runData            RunData
	runDataInitializer func() (RunData, error)
}

type Task struct {
	Name        string
	Run         func(RunData) error
	Skip        func(RunData) (bool, error)
	Tasks       []Task
	RunSubTasks bool
}

type RunData = interface{}

func NewPhase() *Phase {
	return &Phase{
		Tasks: []Task{},
	}
}

func (p *Phase) AppendTask(t Task) {
	p.Tasks = append(p.Tasks, t)
}

func (p *Phase) initData() (RunData, error) {
	if p.runData == nil && p.runDataInitializer != nil {
		var err error
		if p.runData, err = p.runDataInitializer(); err != nil {
			klog.ErrorS(err, "failed to initialize running data")
			return nil, err
		}
	}

	return p.runData, nil
}

func (p *Phase) SetDataInitializer(build func() (RunData, error)) {
	p.runDataInitializer = build
}

func (p *Phase) Run() error {
	runData := p.runData
	if runData == nil {
		if _, err := p.initData(); err != nil {
			return err
		}
	}

	for _, t := range p.Tasks {
		if err := run(t, p.runData); err != nil {
			return err
		}
	}

	return nil
}

func (p *Phase) Init() error {
	runData := p.runData
	if runData == nil {
		if _, err := p.initData(); err != nil {
			return err
		}
	}
	return nil
}

func run(t Task, data RunData) error {
	if t.Skip != nil {
		skip, err := t.Skip(data)
		if err != nil {
			return err
		}
		if skip {
			return nil
		}
	}

	if t.Run != nil {
		if err := t.Run(data); err != nil {
			return err
		}
		if t.RunSubTasks {
			for _, p := range t.Tasks {
				if err := run(p, data); err != nil {
					return err
				}
			}
		}
	}

	return nil
}
