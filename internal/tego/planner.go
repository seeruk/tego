package tego

type Planner struct{}

func NewPlanner() *Planner {
	return &Planner{}
}

func (p *Planner) Plan() (Plan, error) {
	return Plan{}, nil
}
